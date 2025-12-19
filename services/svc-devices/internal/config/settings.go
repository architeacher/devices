package config

import "time"

var (
	ServiceVersion string
	CommitSHA      string
	APIVersion     string
)

const (
	Development = 1 << iota
	Sandbox
	Staging
	Production
)

type (
	ServiceConfig struct {
		App        AppConfig        `json:"app"`
		Logging    LoggingConfig    `json:"logging"`
		Telemetry  TelemetryConfig  `json:"telemetry"`
		GRPCServer GRPCServerConfig `json:"grpc_server"`
		Database   DatabaseConfig   `json:"database"`
		Cache      CacheConfig      `json:"cache"`
	}

	AppConfig struct {
		ServiceName    string `envconfig:"APP_SERVICE_NAME" default:"svc-devices" json:"service_name"`
		ServiceVersion string `envconfig:"APP_SERVICE_VERSION" default:"1.0.0" json:"service_version"`
		CommitSHA      string `envconfig:"APP_COMMIT_SHA" default:"unknown" json:"commit_sha"`
		APIVersion     string `envconfig:"APP_API_VERSION" default:"v1" json:"api_version"`
		Env            string `envconfig:"APP_ENVIRONMENT" default:"development" json:"env"`
	}

	LoggingConfig struct {
		Level     string          `envconfig:"LOG_LEVEL" default:"info" json:"level"`
		Format    string          `envconfig:"LOG_FORMAT" default:"json" json:"format"`
		AccessLog AccessLogConfig `json:"access_log"`
	}

	AccessLogConfig struct {
		Enabled            bool `envconfig:"ACCESS_LOG_ENABLED" default:"true" json:"enabled"`
		LogHealthChecks    bool `envconfig:"ACCESS_LOG_HEALTH_CHECKS" default:"false" json:"log_health_checks"`
		IncludeQueryParams bool `envconfig:"ACCESS_LOG_INCLUDE_QUERY_PARAMS" default:"true" json:"include_query_params"`
	}

	TelemetryConfig struct {
		Enabled        bool    `envconfig:"OTEL_ENABLED" default:"false" json:"enabled"`
		ExporterType   string  `envconfig:"OTEL_EXPORTER" default:"grpc" json:"exporter_type"`
		OTLPEndpoint   string  `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:"" json:"otlp_endpoint"`
		ServiceName    string  `envconfig:"OTEL_SERVICE_NAME" default:"svc-devices" json:"service_name"`
		ServiceVersion string  `envconfig:"OTEL_SERVICE_VERSION" default:"1.0.0" json:"service_version"`
		Metrics        Metrics `json:"metrics"`
		Traces         Traces  `json:"traces"`
	}

	Metrics struct {
		Enabled bool `envconfig:"METRICS_ENABLED" default:"false" json:"enabled"`
	}

	Traces struct {
		Enabled      bool    `envconfig:"TRACES_ENABLED" default:"false" json:"enabled"`
		SamplerRatio float64 `envconfig:"TRACES_SAMPLER_RATIO" default:"1.0" json:"sampler_ratio"`
	}

	GRPCServerConfig struct {
		Host            string        `envconfig:"GRPC_SERVER_HOST" default:"0.0.0.0" json:"host"`
		Port            uint          `envconfig:"GRPC_SERVER_PORT" default:"9090" json:"port"`
		ShutdownTimeout time.Duration `envconfig:"GRPC_SHUTDOWN_TIMEOUT" default:"30s" json:"shutdown_timeout"`
		MaxRecvMsgSize  int           `envconfig:"GRPC_MAX_RECV_MSG_SIZE" default:"4194304" json:"max_recv_msg_size"`
		MaxSendMsgSize  int           `envconfig:"GRPC_MAX_SEND_MSG_SIZE" default:"4194304" json:"max_send_msg_size"`
	}

	DatabaseConfig struct {
		Host            string        `envconfig:"POSTGRES_HOST" default:"postgres" json:"host"`
		Port            uint          `envconfig:"POSTGRES_PORT" default:"5432" json:"port"`
		Database        string        `envconfig:"POSTGRES_DATABASE" default:"devices" json:"database"`
		Username        string        `envconfig:"POSTGRES_USERNAME" default:"postgres" json:"username"`
		Password        string        `envconfig:"POSTGRES_PASSWORD" default:"" json:"password,omitempty"`
		SSLMode         string        `envconfig:"POSTGRES_SSL_MODE" default:"disable" json:"ssl_mode"`
		MaxConnections  int           `envconfig:"POSTGRES_MAX_CONNECTIONS" default:"25" json:"max_connections"`
		MinConnections  int           `envconfig:"POSTGRES_MIN_CONNECTIONS" default:"5" json:"min_connections"`
		ConnectTimeout  time.Duration `envconfig:"POSTGRES_CONNECT_TIMEOUT" default:"10s" json:"connect_timeout"`
		MaxConnLifetime time.Duration `envconfig:"POSTGRES_MAX_CONN_LIFETIME" default:"1h" json:"max_conn_lifetime"`
		MaxConnIdleTime time.Duration `envconfig:"POSTGRES_MAX_CONN_IDLE_TIME" default:"30m" json:"max_conn_idle_time"`
	}

	CacheConfig struct {
		Address  string `envconfig:"KEYDB_ADDR" default:"keydb:6379" json:"address"`
		Password string `envconfig:"KEYDB_PASSWORD" default:"" json:"password,omitempty"`
	}
)

func (c *ServiceConfig) GetEnvironment() int {
	switch c.App.Env {
	case "production", "prod":
		return Production
	case "staging", "stg":
		return Staging
	case "sandbox", "sbx":
		return Sandbox
	default:
		return Development
	}
}

func (c *ServiceConfig) IsProduction() bool {
	return c.GetEnvironment() == Production
}
