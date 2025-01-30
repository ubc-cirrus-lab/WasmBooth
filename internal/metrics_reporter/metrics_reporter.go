package metrics_reporter

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"webserver/internal/cgroup_manager"
	"webserver/internal/config"
	"webserver/internal/metrics_collector"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

type MetricsReporter struct {
	Config           *config.MetricsReporterConfig
	MetricsCollector *metrics_collector.MetricsCollector
}

func (mr *MetricsReporter) connectWithRetry(addr string) (*websocket.Conn, error) {
	var conn *websocket.Conn
	var err error

	for i := 0; i < mr.Config.MaxRetries; i++ {
		conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
		if err == nil {
			return conn, nil
		}

		time.Sleep(time.Duration(mr.Config.RetryDelaySec) * time.Second)
	}

	return nil, fmt.Errorf("could not connect to server after %d attempts: %v", mr.Config.MaxRetries, err)
}

func (mr *MetricsReporter) Run() {
	addr := mr.Config.QPHost + ":" + strconv.Itoa(mr.Config.QPPort)
	conn, err := mr.connectWithRetry(addr)
	if err != nil {
		slog.Error("Failed to connect to Queue Proxy", "address", addr, "reason", err)
	}
	defer conn.Close()

	reportTicker := time.NewTicker(time.Duration(mr.Config.ReportingPeriodMS) * time.Millisecond)
	defer reportTicker.Stop()

	for range reportTicker.C {
		mr.MetricsCollector.Update()

		averageUtilization := mr.MetricsCollector.GetAverageCPUUtilization()

		metrics := &cgroup_manager.SuperPodMetrics{
			CpuUtilization: averageUtilization,
		}

		// slog.Debug("Sending metrics to Queue Proxy", "averageUtilization", averageUtilization)

		buffer, err := proto.Marshal(metrics)
		if err != nil {
			slog.Error("Failed to marshal metrics", "reason", err)
			return
		}

		err = conn.WriteMessage(websocket.BinaryMessage, buffer)
		if err != nil {
			slog.Error("Failed to write message to Queue Proxy", "reason", err)
			return
		}
	}
}
