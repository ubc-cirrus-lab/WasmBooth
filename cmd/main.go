package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"webserver/internal/cgroup_manager"
	"webserver/internal/config"
	"webserver/internal/healthcheck"
	"webserver/internal/http_server"
	"webserver/internal/metrics_collector"
	"webserver/internal/metrics_reporter"

	"github.com/ilyakaznacheev/cleanenv"
	_ "go.uber.org/automaxprocs"
)

func InitLogger(logLevel string) {
	level := slog.LevelInfo
	if logLevel == "debug" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

func getContainerID(podUID string) string {
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(podUID, "-", "_") + ".slice"
	podCgroupPath := filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup)

	criDirs, err := filepath.Glob(filepath.Join(podCgroupPath, "cri-containerd-*"))
	if err != nil {
		slog.Error("Failed to get CRI directories", "reason", err)
		return ""
	}

	slog.Debug("Num of CRI dirs", "num", len(criDirs), "pattern", filepath.Join(podCgroupPath, "cri-containerd-*"))

	for _, dir := range criDirs {
		file := filepath.Join(dir, "cgroup.procs")

		content, err := os.ReadFile(file)
		if err != nil {
			slog.Error("Failed to read file %s: %v", file, err)
			continue
		}

		firstLine := strings.TrimSpace(string(content))
		if firstLine == "1" {
			return filepath.Base(dir)
		}
	}

	return ""
}

func main() {
	InitLogger(os.Getenv("LOG_LEVEL"))

	podUID := os.Getenv("POD_UID")
	containerID := getContainerID(podUID)

	// MetricsReporterConfig := &config.MetricsReporterConfig{
	// 	ReportingPeriod: 1 * time.Second,
	// 	QPHost:          "ws://127.0.0.1",
	// 	QPPort:          9096,
	// 	MaxRetries:      5,
	// 	RetryDelay:      500 * time.Millisecond,
	// }

	var metricCollectorConfig config.MetricsCollector
	err := cleanenv.ReadEnv(&metricCollectorConfig)
	if err != nil {
		log.Fatal(err)
	}

	var MetricsReporterConfig config.MetricsReporterConfig
	err = cleanenv.ReadEnv(&MetricsReporterConfig)
	if err != nil {
		log.Fatal(err)
	}

	var webServerConfig config.WebServerConfig
	err = cleanenv.ReadEnv(&webServerConfig)
	if err != nil {
		log.Fatal(err)
	}

	var healthCheckConfig config.HealthCheckConfig
	err = cleanenv.ReadEnv(&healthCheckConfig)
	if err != nil {
		log.Fatal(err)
	}

	cgroupManagerConfig := &config.CgroupManagerConfig{
		PodUID:      podUID,
		ContainerID: containerID,
	}

	cgroupManager := &cgroup_manager.CgroupManager{
		Config: cgroupManagerConfig,
	}
	cgroupManager.Init()

	metricsCollector := metrics_collector.MetricsCollector{
		Config:        &metricCollectorConfig,
		CgroupManager: cgroupManager,
	}
	metricsCollector.Init()

	metricsReporter := metrics_reporter.MetricsReporter{
		Config:           &MetricsReporterConfig,
		MetricsCollector: &metricsCollector,
	}

	go metricsReporter.Run()
	slog.Info("Started the Metrics Reporter")

	server := http_server.WebServer{
		Config:        &webServerConfig,
		ReadyWEXs:     make(map[string][]string),
		CgroupManager: cgroupManager,
	}

	healthcheck.Init(&healthCheckConfig, &server)
	go server.Start()
	slog.Info("Started the Web Server", "address", server.Config.Host+":"+strconv.Itoa(server.Config.Port), "pid", os.Getpid(), "cgroup", cgroupManager.GetContainerCgroupPath())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("Stopped the server gracefully")
}
