package healthcheck

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
	"webserver/internal/config"
	"webserver/internal/http_server"

	"github.com/heptiolabs/healthcheck"
)

type HealthChecker struct {
	Config            *config.HealthCheckConfig
	healthcheckServer *http.Server
	wasmServer        *http_server.WebServer
}

func Init(config *config.HealthCheckConfig, server *http_server.WebServer) {
	healthCheckServer := HealthChecker{
		Config:     config,
		wasmServer: server,
	}
	go healthCheckServer.ServeHealthCheck()
}

func portIsAvailable(serverAddr string) error {
	timeout := 5 * time.Second

	conn, err := net.DialTimeout("tcp", serverAddr, timeout)

	if err != nil {
		return err
	}

	if conn != nil {
		err = conn.Close()
	}

	return err
}

func (healthChecker *HealthChecker) ServeHealthCheck() {
	healthCheckHandler := healthcheck.NewHandler()

	healthCheckHandler.AddReadinessCheck("webserver-readiness", func() error {
		return healthChecker.wasmServer.IsBusy()
	})

	healthChecker.healthcheckServer = &http.Server{
		Addr:    healthChecker.Config.Host + ":" + strconv.Itoa(healthChecker.Config.Port),
		Handler: healthCheckHandler,
	}
	err := healthChecker.healthcheckServer.ListenAndServe()

	if err != nil {
		slog.Error("HealthChecker couldn't serve", "reason", err.Error())
	}
}
