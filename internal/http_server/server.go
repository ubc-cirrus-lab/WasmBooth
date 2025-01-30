package http_server

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"webserver/internal/cgroup_manager"
	"webserver/internal/config"

	"github.com/bytecodealliance/wasmtime-go/v24"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/second-state/WasmEdge-go/wasmedge"
	bindgen "github.com/second-state/wasmedge-bindgen/host/go"
	"golang.org/x/exp/rand"
)

const (
	DefaultMemoryLimit = "200" // MB
	DefaultCPULimit    = "500" // Milicores
)

type WasmThreadResult struct {
	Output string
	Err    error
}

type WebServer struct {
	Config               *config.WebServerConfig
	ReadyWEXs            map[string][]string
	WEXs                 []string
	CgroupManager        *cgroup_manager.CgroupManager
	MemUtilizationWindow *list.List
	CurrentRequests      int32
}

type PostRequestBody struct {
	Parameter string `json:"parameter"`
}

func (ws *WebServer) Start() {
	rand.Seed(uint64(time.Now().UnixNano()))
	ws.MemUtilizationWindow = list.New()
	router := mux.NewRouter()

	router.HandleFunc("/{wasm_file}", ws.HandleGet).Methods("GET")
	router.HandleFunc("/{wasm_file}", ws.HandlePost).Methods("POST")

	http.Handle("/", router)
	if err := http.ListenAndServe(ws.Config.Host+":"+strconv.Itoa(ws.Config.Port), nil); err != nil {
		slog.Error("Failed to start Server", "reason", err)
	}
}

func (ws *WebServer) HandleGet(w http.ResponseWriter, req *http.Request) {
	slog.Debug("Received a GET request")
	atomic.AddInt32(&ws.CurrentRequests, 1)
	defer atomic.AddInt32(&ws.CurrentRequests, -1)

	ws.HandleRequest(w, req, "")
}

func (ws *WebServer) HandlePost(w http.ResponseWriter, req *http.Request) {
	slog.Info("Received a POST request")
	atomic.AddInt32(&ws.CurrentRequests, 1)
	defer atomic.AddInt32(&ws.CurrentRequests, -1)

	var requestBody PostRequestBody

	// Decode the JSON body into the struct
	err := json.NewDecoder(req.Body).Decode(&requestBody)
	if err != nil {
		slog.Debug("Invalid/Empty request body", "reason", err)
	}

	// Call HandleRequest() with provided WASM parameter
	ws.HandleRequest(w, req, requestBody.Parameter)
}

func (ws *WebServer) HandleRequest(w http.ResponseWriter, req *http.Request, wasmParam string) {
	start := time.Now()
	runtime.LockOSThread()
	slog.Debug("Locked OS thread", "time", time.Since(start))

	handlerID := strconv.Itoa(syscall.Gettid())
	requestID := uuid.New().String()
	wasmFile := mux.Vars(req)["wasm_file"]

	cpuLimit := "1000"
	memLimit := "1000"

	// Check validity of the request
	if !ws.IsValidRequest(req.Header) {
		// w.WriteHeader(http.StatusBadRequest)
		slog.Info("Invalid request, not specified resources", "handler_id", handlerID, "wasm_file", wasmFile)
		// w.Write([]byte("Invalid request\n"))

		// beforeLock := time.Now()
		// runtime.UnlockOSThread()
		// slog.Debug("Unlocked OS thread", "time", time.Since(beforeLock))
		// return
	}

	var finalWasmOutput string
	var finalStatus int

	wasmOutput, timesData, err := ws.HandleThreadExecution(handlerID, requestID, wasmFile, cpuLimit, memLimit, wasmParam)

	if err != nil {
		slog.Error("Failed to run WASM thread", "reason", err)
		finalStatus, finalWasmOutput = http.StatusInternalServerError, "Failed to run WASM module\n"
	} else {
		finalStatus, finalWasmOutput = http.StatusOK, wasmOutput
	}

	beforeLock := time.Now()
	runtime.UnlockOSThread()
	slog.Debug("Unlocked OS thread", "time", time.Since(beforeLock))
	slog.Debug("Done with a request", "handler_id", handlerID, "request_id", requestID, "time", time.Since(start))
	w.Header().Set("Content-Type", "text/plain")
	for key, value := range timesData {
		w.Header().Set(key, value)
	}

	w.WriteHeader(finalStatus)
	w.Write([]byte(fmt.Sprintf("WASM output: %s", strings.TrimRight(finalWasmOutput, "\x00"))))
}

func (ws *WebServer) IsValidRequest(headers http.Header) bool {
	return headers.Get("cpu_quota") != "" && headers.Get("Memory-Request") != ""
}

func (ws *WebServer) HandleThreadExecution(handlerID, requestID, wasmFile, memLimit, cpuLimit, wasmModuleParam string) (string, map[string]string, error) {
	// Acquire a cgorup with according cpu/memory resource limits
	beforeCgroupCreateTime := time.Now()
	if ws.Config.EnableCgroups {
		ws.CgroupManager.Acquire(requestID, cpuLimit, memLimit)
	}
	cgroupCreationTime := time.Since(beforeCgroupCreateTime)

	// Assign a cgorup with according cpu/memory resource limits
	beforeCgroupAssignTime := time.Now()
	if ws.Config.EnableCgroups {
		ws.CgroupManager.Assign(requestID, handlerID)
	}
	cgroupAssignTime := time.Since(beforeCgroupAssignTime)

	// Run WASM thread
	beforeExecutionTime := time.Now()
	wasmThreadOutput := ws.RunWasmThread(handlerID, requestID, wasmFile, wasmModuleParam, memLimit)
	executionTime := time.Since(beforeExecutionTime)

	// Delete the cgroup after the execution
	if ws.Config.EnableCgroups {
		ws.CgroupManager.Release(requestID)
	}

	timesData := map[string]string{
		"Cgroup-Creation-Time": strconv.FormatInt(cgroupCreationTime.Milliseconds(), 10),
		"Cgroup-Assign-Time":   strconv.FormatInt(cgroupAssignTime.Milliseconds(), 10),
		"Execution-Time":       strconv.FormatInt(executionTime.Milliseconds(), 10),
		"Num-Current-Requests": strconv.Itoa(int(ws.CurrentRequests)),
		"Pod":                  ws.CgroupManager.Config.PodUID,
	}

	if wasmThreadOutput.Err != nil {
		return "", timesData, wasmThreadOutput.Err
	}

	return wasmThreadOutput.Output, timesData, nil
}

func (ws *WebServer) LoadModule(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (ws *WebServer) RunWasmThread(handlerID, requestID, wasmFile string, wasmModuleParam string, maxMemory string) WasmThreadResult {
	if ws.Config.WasmRuntime == "wasmtime" {
		return ws.RunWasmtime(handlerID, requestID, wasmFile, wasmModuleParam, maxMemory)
	} else if ws.Config.WasmRuntime == "wasmedge" {
		return ws.RunWasmedge(handlerID, requestID, wasmFile, wasmModuleParam, maxMemory)
	} else {
		return ws.RunWasmtime(handlerID, requestID, wasmFile, wasmModuleParam, maxMemory)
	}
}

func (ws *WebServer) RunWasmtime(handlerID, requestID, wasmFile string, wasmModuleParam string, maxMemory string) WasmThreadResult {
	// Use Wasmtime to execute "wasmFile"
	slog.Info("Start WASM thread", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory)
	dir, err := os.MkdirTemp("", "out")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	stdoutPath := filepath.Join(dir, requestID)
	stdinPath := filepath.Join(dir, requestID+"stdin")

	// Write WASM instance parameter to stdin
	stdin, err := os.Create(stdinPath)
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	_, err = stdin.WriteString(wasmModuleParam)
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	defer stdin.Close()

	engine := wasmtime.NewEngine()

	slog.Debug("Loaded engine", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory)

	beforeModuleCreation := time.Now()
	module, err := wasmtime.NewModuleDeserializeFile(engine, filepath.Join("functions", wasmFile))
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}
	slog.Debug("Created module", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory, "time", time.Since(beforeModuleCreation))

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	wasiConfig := wasmtime.NewWasiConfig()
	wasiConfig.SetStdoutFile(stdoutPath)
	wasiConfig.SetStdinFile(stdinPath)
	store := wasmtime.NewStore(engine)
	store.SetWasi(wasiConfig)

	// Limit the WASM thread's linear memory usage (in bytes)
	store.Limiter(getMemoryInBytes(maxMemory), -1, 1, -1, 1)
	slog.Debug("Limited memory", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory)
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	// Run the function
	beforeCall := time.Now()
	nom := instance.GetFunc(store, "_start")
	_, err = nom.Call(store)
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	slog.Debug("Waiting for output", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory, "time", time.Since(beforeCall))

	// Print WASM stdout
	out, err := os.ReadFile(stdoutPath)
	if err != nil {
		return WasmThreadResult{Output: "", Err: err}
	}

	slog.Debug("Executed WASM function", "handler_id", strconv.Itoa(syscall.Gettid()), "memory_limit", maxMemory)

	return WasmThreadResult{Output: string(out) + "\n", Err: nil}
}

func (ws *WebServer) RunWasmedge(handlerID, requestID, wasmFile string, wasmModuleParam string, maxMemory string) WasmThreadResult {
	wasmedge.SetLogErrorLevel()

	conf := wasmedge.NewConfigure(wasmedge.WASI)
	conf.SetMaxMemoryPage(uint(getMemoryInWasmPages(maxMemory)))
	slog.Debug("Max memory is configured", "max value (Wasm pages)", getMemoryInWasmPages(maxMemory))

	vm := wasmedge.NewVMWithConfig(conf)

	var wasi = vm.GetImportModule(wasmedge.WASI)
	wasi.InitWasi(
		nil,
		nil,
		nil,
	)

	err := vm.LoadWasmFile(filepath.Join("functions", wasmFile))
	if err != nil {
		slog.Error("Load WASM from file failed.", "reason", err.Error())
		vm.Release()
		conf.Release()
		return WasmThreadResult{Output: "", Err: err}
	}

	err = vm.Validate()
	if err != nil {
		slog.Debug("Wasmedge validation failed.", "reason", err.Error())
		vm.Release()
		conf.Release()
		return WasmThreadResult{Output: "", Err: err}
	}

	bg := bindgen.New(vm)
	bg.Instantiate()

	res, _, err := bg.Execute("_main")
	if err != nil {
		slog.Error("Run failed", "reason", err.Error())
		bg.Release()
		vm.Release()
		conf.Release()
		return WasmThreadResult{Output: "", Err: err}
	}

	bg.Release()
	vm.Release()
	conf.Release()

	return WasmThreadResult{Output: res[0].(string), Err: err}
}

func getMemoryInBytes(memory string) int64 {
	maxMemoryInt, err := strconv.Atoi(memory)
	if err != nil {
		return 0
	}

	maxMemoryBytes := int64(65536 * 16 * maxMemoryInt)
	return maxMemoryBytes
}

func getMemoryInWasmPages(memory string) int64 {
	cleanedMemory := strings.TrimSpace(memory)
	memoryMB, err := strconv.ParseInt(cleanedMemory, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Invalid memory format: %v", err))
	}

	memoryBytes := memoryMB * 1024 * 1024
	wasmPageSize := int64(64 * 1024)
	return int64(math.Ceil(float64(memoryBytes) / float64(wasmPageSize)))
}

func (ws *WebServer) IsBusy() error {
	memoryUsageMB := ws.CgroupManager.GetCurrentMemoryUsage()
	memoryUtilization := memoryUsageMB / ws.Config.MemoryLimit

	ws.MemUtilizationWindow.PushBack(memoryUtilization)

	// GC if utilization is high
	if memoryUtilization > ws.Config.GCUtilizationTreshold {
		runtime.GC()
		slog.Debug("GCed", "memoryUsageMB", memoryUsageMB, "memoryUtilization", memoryUtilization)
	}

	if ws.MemUtilizationWindow.Len() > ws.Config.ReadinessWindow {
		ws.MemUtilizationWindow.Remove(ws.MemUtilizationWindow.Front())
	}

	// slog.Debug("Got the memoryUsageMB", "memory_usage_mb", memoryUsageMB, "window_len", ws.MemUtilizationWindow.Len())

	randomNum := rand.Intn(100)
	if ws.GetAvgMemoryUtilization() > ws.Config.ReadinessUtilizationTreshold && float64(randomNum) > ws.Config.ReadinessUtilizationTreshold {
		return errors.New("not ready; Memory utilization exceeded the threshold")
	}

	return nil
}

func (ws *WebServer) GetAvgMemoryUtilization() float64 {
	if ws.MemUtilizationWindow.Len() == 0 {
		return 0.0
	}

	sum := 0.0
	for e := ws.MemUtilizationWindow.Front(); e != nil; e = e.Next() {
		sum = e.Value.(float64)
	}

	return sum / float64(ws.MemUtilizationWindow.Len())
}
