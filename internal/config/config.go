package config

type CgroupManagerConfig struct {
	PodUID      string
	ContainerID string
}

type WebServerConfig struct {
	Host                         string  `env-required:"true" env:"WEBSERVER_HOST"`
	Port                         int     `env-required:"true" env:"WEBSERVER_PORT"`
	WasmRuntime                  string  `env-required:"true" env:"WASM_RUNTIME"`
	ReadinessWindow              int     `env-required:"true" env:"READINESS_WINDOW"`
	ReadinessUtilizationTreshold float64 `env-required:"true" env:"READINESS_UTILIZATION_TRESHOLD"`
	ReadinessRandTreshold        int     `env-required:"true" env:"READINESS_RAND_TRESHOLD"`
	GCUtilizationTreshold        float64 `env-required:"true" env:"GC_UTILIZATION_TRESHOLD"`
	MemoryLimit                  float64 `env-required:"true" env:"MEMORY_LIMIT"`
}

type HealthCheckConfig struct {
	Host string `env-required:"true" env:"HEALTHCHECK_HOST"`
	Port int    `env-required:"true" env:"HEALTHCHECK_PORT"`
}

type MetricsReporterConfig struct {
	ReportingPeriodMS int    `env-required:"true" env:"REPORTING_PERIOD_MS"`
	QPHost            string `env-required:"true" env:"QP_HOST"`
	QPPort            int    `env-required:"true" env:"QP_PORT"`
	MaxRetries        int    `env-required:"true" env:"MAX_RETRIES"`
	RetryDelaySec     int    `env-required:"true" env:"RETRY_DELAY_SEC"`
}

type MetricsCollector struct {
	MetricsCollectionWindow int `env-required:"true" env:"METRICS_COLLECTION_WINDOW"`
}

type MinioConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
}
