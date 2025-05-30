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
	slog.Info("Received a request")
	ws.HandleRequest(w, req, "")
}

func (ws *WebServer) HandlePost(w http.ResponseWriter, req *http.Request) {
	slog.Info("Received a request")
	var requestBody PostRequestBody

	// Decode the JSON body into the struct
	err := json.NewDecoder(req.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	// Check validity of the request
	if !ws.IsValidRequest(req.Header) {
		w.WriteHeader(http.StatusBadRequest)
		slog.Info("Invalid request, not specified resources", "handler_id", handlerID, "wasm_file", wasmFile)
		w.Write([]byte("Invalid request\n"))

		beforeLock := time.Now()
		runtime.UnlockOSThread()
		slog.Debug("Unlocked OS thread", "time", time.Since(beforeLock))
		return
	}

	var finalWasmOutput string
	var finalStatus int

	wasmOutput, err := ws.HandleThreadExecution(handlerID, requestID, wasmFile, req.Header, wasmParam)

	if err != nil {
		slog.Error("Failed to run WASM thread", "reason", err)
		finalStatus, finalWasmOutput = http.StatusInternalServerError, "Failed to run WASM module\n"
	} else {
		finalStatus, finalWasmOutput = http.StatusOK, wasmOutput
	}

	beforeLock := time.Now()
	runtime.UnlockOSThread()
	slog.Debug("Unlocked OS thread", "time", time.Since(beforeLock))
	slog.Info("Done with a request", "handler_id", handlerID, "request_id", requestID, "time", time.Since(start))
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(finalStatus)
	w.Write([]byte(fmt.Sprintf("WASM output: %s", strings.TrimRight(finalWasmOutput, "\x00"))))
}

func (ws *WebServer) IsValidRequest(headers http.Header) bool {
	return headers.Get("cpu_quota") != "" && headers.Get("Memory-Request") != ""
}

func (ws *WebServer) HandleThreadExecution(handlerID, requestID, wasmFile string, headers http.Header, wasmModuleParam string) (string, error) {
	// Acquire a cgorup with according cpu/memory resource limits
	ws.CgroupManager.Acquire(requestID, headers.Get("cpu_quota"), headers.Get("Memory-Request"))

	// Run WASM thread
	wasmThreadOutput := ws.RunWasmThread(handlerID, requestID, wasmFile, wasmModuleParam, headers.Get("Memory-Request"))

	// Delete the cgroup after the execution
	ws.CgroupManager.Release(requestID)

	if wasmThreadOutput.Err != nil {
		return "", wasmThreadOutput.Err
	}

	return wasmThreadOutput.Output, nil
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
	// Assign a cgorup with according cpu/memory resource limits
	ws.CgroupManager.Assign(requestID, handlerID)

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

	maxMemoryInt, _ := strconv.Atoi(maxMemory)
	preallocatedSize := int32(float64(maxMemoryInt) * ws.Config.MemPreAllocationRatio)

	conf := wasmedge.NewConfigure(wasmedge.WASI)
	conf.SetMaxMemoryPage(uint(getMemoryInWasmPages(maxMemory)))
	slog.Debug("Memory is configured", "max_value (Wasm pages)", getMemoryInWasmPages(maxMemory), "preallocated_size (mb)", preallocatedSize)
	// defer conf.Release()

	vm := wasmedge.NewVMWithConfig(conf)
	// defer vm.Release()

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
	// defer bg.Release()

	// Pre-allocate memory
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	startPreAllocTime := time.Now()
	allocateResult, _ := vm.Execute("allocate", int32(getMemoryInBytes(strconv.Itoa(int(preallocatedSize)))+1))
	inputPointer := allocateResult[0].(int32)
	memData, _ := mem.GetData(uint(inputPointer), uint(getMemoryInBytes(strconv.Itoa(int(preallocatedSize)))+1))
	for i := range memData {
		memData[i] = byte(1)
	}
	memData[getMemoryInBytes(strconv.Itoa(int(preallocatedSize)))] = 0
	slog.Debug("Memory is pre-allocated", "time", time.Since(startPreAllocTime))

	vm.Execute("deallocate", inputPointer, int32(getMemoryInBytes(strconv.Itoa(int(preallocatedSize)))+1))

	// Assign a cgorup with according cpu/memory resource limits
	ws.CgroupManager.Assign(requestID, handlerID)

	// Execute the WASM module
	res, err := vm.Execute("_main", inputPointer, preallocatedSize, int32(time.Since(startPreAllocTime).Milliseconds()))
	if err != nil {
		slog.Error("Run failed", "reason", err.Error())
		vm.Release()
		conf.Release()
		bg.Release()
		return WasmThreadResult{Output: "", Err: err}
	}

	// Retrieve the output
	outputPointer := res[0].(int32)
	pageSize := mem.GetPageSize()
	memData, _ = mem.GetData(uint(0), uint(pageSize*65536))
	nth := 0
	var output strings.Builder

	for {
		if memData[int(outputPointer)+nth] == 0 {
			break
		}

		output.WriteByte(memData[int(outputPointer)+nth])
		nth++
	}
	lengthOfOutput := nth

	vm.Execute("deallocate", outputPointer, int32(lengthOfOutput+1))

	vm.Release()
	conf.Release()
	bg.Release()

	return WasmThreadResult{Output: output.String(), Err: err}
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
