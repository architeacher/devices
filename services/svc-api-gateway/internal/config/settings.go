package config

import (
	"time"
)

// Compile time variables are set by -ldflags.
var (
	ServiceVersion string
	CommitSHA      string
)

const (
	Development = 1 << iota
	Sandbox
	Staging
	Production
)

type (
	ServiceConfig struct {
		App                   App                   `json:"app"`
		SecretsStorage        SecretsStorage        `json:"secrets_storage"`
		HTTPServer            HTTPServer            `json:"http_server_server"`
		Auth                  Auth                  `json:"auth"`
		DevicesGRPCClient     DevicesGRPCClient     `json:"devices_grpc_client"`
		Backoff               Backoff               `json:"backoff"`
		Cache                 Cache                 `json:"cache"`
		ThrottledRateLimiting ThrottledRateLimiting `json:"throttled_rate_limiting"`
		Idempotency           Idempotency           `json:"idempotency"`
		Deprecation           Deprecation           `json:"deprecation"`
		Logging               Logging               `json:"logging"`
		Telemetry             Telemetry             `json:"telemetry"`
	}

	App struct {
		ServiceName string      `envconfig:"APP_SERVICE_NAME" default:"svc-api-gateway" json:"service_name"`
		APIVersion  string      `envconfig:"APP_API_VERSION" default:"v1" json:"api_version"`
		Env         Environment `json:"environment"`
	}

	Environment struct {
		Name string `envconfig:"APP_ENVIRONMENT" default:"development" json:"env"`
	}

	SecretsStorage struct {
		Enabled       bool          `envconfig:"VAULT_ENABLED" default:"true" json:"enabled"`
		Address       string        `envconfig:"VAULT_ADDRESS" default:"http://vault:8200" json:"address"`
		Token         string        `envconfig:"VAULT_TOKEN" default:"" json:"token,omitempty"`
		RoleID        string        `envconfig:"VAULT_ROLE_ID" default:"" json:"role_id,omitempty"`
		SecretID      string        `envconfig:"VAULT_SECRET_ID" default:"" json:"secret_id,omitempty"`
		AuthMethod    string        `envconfig:"VAULT_AUTH_METHOD" default:"token" json:"auth_method"`
		MountPath     string        `envconfig:"VAULT_MOUNT_PATH" default:"svc-api-gateway" json:"mount_path"`
		Namespace     string        `envconfig:"VAULT_NAMESPACE" default:"" json:"namespace,omitempty"`
		Timeout       time.Duration `envconfig:"VAULT_TIMEOUT" default:"30s" json:"timeout"`
		MaxRetries    uint          `envconfig:"VAULT_MAX_RETRIES" default:"3" json:"max_retries"`
		TLSSkipVerify bool          `envconfig:"VAULT_TLS_SKIP_VERIFY" default:"false" json:"tls_skip_verify"`
		PollInterval  time.Duration `envconfig:"VAULT_POLL_INTERVAL" default:"24h" json:"poll_interval"`
	}

	HTTPServer struct {
		Host            string        `envconfig:"HTTP_SERVER_HOST" default:"0.0.0.0" json:"host"`
		Port            uint          `envconfig:"HTTP_SERVER_PORT" default:"8088" json:"port"`
		ReadTimeout     time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"15s" json:"read_timeout"`
		WriteTimeout    time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"15s" json:"write_timeout"`
		IdleTimeout     time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"60s" json:"idle_timeout"`
		ShutdownTimeout time.Duration `envconfig:"HTTP_SHUTDOWN_TIMEOUT" default:"30s" json:"shutdown_timeout"`
	}

	Auth struct {
		Enabled        bool          `envconfig:"AUTH_ENABLED" default:"true" json:"enabled"`
		SecretKey      string        `envconfig:"AUTH_SECRET_KEY" default:"" json:"secret_key,omitempty"`
		ValidIssuers   []string      `envconfig:"AUTH_VALID_ISSUERS" default:"svc-api-gateway,auth-service" json:"valid_issuers"`
		TokenExpiry    time.Duration `envconfig:"AUTH_TOKEN_EXPIRY" default:"1h" json:"token_expiry"`
		SkipPaths      []string      `envconfig:"AUTH_SKIP_PATHS" default:"/v1/health,/v1/liveness,/v1/readiness" json:"skip_paths"`
		PasetoKeyPath  string        `envconfig:"AUTH_PASETO_KEY_PATH" default:"" json:"paseto_key_path"`
		FallbackKeyHex string        `envconfig:"AUTH_FALLBACK_KEY_HEX" default:"" json:"fallback_key_hex,omitempty"`
	}

	DevicesGRPCClient struct {
		Address        string               `envconfig:"DEVICES_GRPC_ADDRESS" default:"svc-devices:9090" json:"address"`
		Timeout        time.Duration        `envconfig:"DEVICES_TIMEOUT" default:"30s" json:"timeout"`
		MaxRetries     uint                 `envconfig:"DEVICES_MAX_RETRIES" default:"3" json:"max_retries"`
		CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
		TLS            TLSConfig            `json:"tls"`
	}

	TLSConfig struct {
		Enabled  bool   `envconfig:"DEVICES_TLS_ENABLED" default:"false" json:"enabled"`
		CertFile string `envconfig:"DEVICES_TLS_CERT_FILE" default:"" json:"cert_file,omitempty"`
		CAFile   string `envconfig:"DEVICES_TLS_CA_FILE" default:"" json:"ca_file,omitempty"`
	}

	CircuitBreakerConfig struct {
		Enabled          bool          `envconfig:"DEVICES_CB_ENABLED" default:"true" json:"enabled"`
		MaxRequests      uint          `envconfig:"DEVICES_CB_MAX_REQUESTS" default:"5" json:"max_requests"`
		Interval         time.Duration `envconfig:"DEVICES_CB_INTERVAL" default:"60s" json:"interval"`
		Timeout          time.Duration `envconfig:"DEVICES_CB_TIMEOUT" default:"30s" json:"timeout"`
		FailureThreshold uint          `envconfig:"DEVICES_CB_FAILURE_THRESHOLD" default:"5" json:"failure_threshold"`
	}

	Backoff struct {
		BaseDelay  time.Duration `envconfig:"BACKOFF_BASE_DELAY" default:"1s" json:"base_delay"`
		Multiplier float64       `envconfig:"BACKOFF_MULTIPLIER" default:"1.5" json:"multiplier"`
		Jitter     float64       `envconfig:"BACKOFF_JITTER" default:"0.3" json:"jitter"`
		MaxDelay   time.Duration `envconfig:"BACKOFF_MAX_DELAY" default:"10s" json:"max_delay"`
	}

	Cache struct {
		Address       string        `envconfig:"CACHE_ADDRESS" default:"keydb:6379" json:"address"`
		Password      string        `envconfig:"CACHE_PASSWORD" default:"" json:"password,omitempty"`
		DB            uint          `envconfig:"CACHE_DB" default:"0" json:"db"`
		PoolSize      uint          `envconfig:"CACHE_POOL_SIZE" default:"10" json:"pool_size"`
		MinIdleConns  uint          `envconfig:"CACHE_MIN_IDLE_CONNS" default:"3" json:"min_idle_conns"`
		DialTimeout   time.Duration `envconfig:"CACHE_DIAL_TIMEOUT" default:"5s" json:"dial_timeout"`
		ReadTimeout   time.Duration `envconfig:"CACHE_READ_TIMEOUT" default:"3s" json:"read_timeout"`
		WriteTimeout  time.Duration `envconfig:"CACHE_WRITE_TIMEOUT" default:"3s" json:"write_timeout"`
		PoolTimeout   time.Duration `envconfig:"CACHE_POOL_TIMEOUT" default:"5s" json:"pool_timeout"`
		MaxRetries    uint          `envconfig:"CACHE_MAX_RETRIES" default:"3" json:"max_retries"`
		DefaultExpiry time.Duration `envconfig:"CACHE_DEFAULT_EXPIRY" default:"24h" json:"default_expiry"`
	}

	ThrottledRateLimiting struct {
		Enabled            bool          `envconfig:"RATE_LIMITING_ENABLED" default:"true" json:"enabled"`
		RequestsPerSecond  uint          `envconfig:"RATE_LIMITING_REQUESTS_PER_SECOND" default:"10" json:"requests_per_second"`
		BurstSize          uint          `envconfig:"RATE_LIMITING_BURST_SIZE" default:"20" json:"burst_size"`
		WindowDuration     time.Duration `envconfig:"RATE_LIMITING_WINDOW_DURATION" default:"5m" json:"window_duration"`
		EnableIPLimiting   bool          `envconfig:"RATE_LIMITING_ENABLE_IP_LIMITING" default:"true" json:"enable_ip_limiting"`
		EnableUserLimiting bool          `envconfig:"RATE_LIMITING_ENABLE_USER_LIMITING" default:"true" json:"enable_user_limiting"`
		CleanupInterval    time.Duration `envconfig:"RATE_LIMITING_CLEANUP_INTERVAL" default:"1m" json:"cleanup_interval"`
		MaxKeys            uint          `envconfig:"RATE_LIMITING_MAX_KEYS" default:"1000" json:"max_keys"`
		SkipPaths          []string      `envconfig:"RATE_LIMITING_SKIP_PATHS" default:"/v1/health,/v1/liveness,/v1/readiness" json:"skip_paths"`
		GracefulDegraded   bool          `envconfig:"RATE_LIMITING_GRACEFUL_DEGRADED" default:"true" json:"graceful_degraded"`
	}

	Idempotency struct {
		Enabled          bool          `envconfig:"IDEMPOTENCY_ENABLED" default:"true" json:"enabled"`
		CacheTTL         time.Duration `envconfig:"IDEMPOTENCY_CACHE_TTL" default:"24h" json:"cache_ttl"`
		LockTTL          time.Duration `envconfig:"IDEMPOTENCY_LOCK_TTL" default:"30s" json:"lock_ttl"`
		RequiredMethods  []string      `envconfig:"IDEMPOTENCY_REQUIRED_METHODS" default:"POST" json:"required_methods"`
		HeaderName       string        `envconfig:"IDEMPOTENCY_HEADER" default:"Idempotency-Key" json:"header_name"`
		ReplayedHeader   string        `envconfig:"IDEMPOTENCY_REPLAYED_HEADER" default:"Idempotent-Replayed" json:"replayed_header"`
		GracefulDegraded bool          `envconfig:"IDEMPOTENCY_GRACEFUL_DEGRADED" default:"true" json:"graceful_degraded"`
	}

	Deprecation struct {
		Enabled       bool   `envconfig:"API_DEPRECATION_ENABLED" default:"false" json:"enabled"`
		SunsetDate    string `envconfig:"API_SUNSET_DATE" default:"" json:"sunset_date"`
		SuccessorPath string `envconfig:"API_SUCCESSOR_PATH" default:"" json:"successor_path"`
	}

	Logging struct {
		Level     string    `envconfig:"LOG_LEVEL" default:"info" json:"level"`
		Format    string    `envconfig:"LOG_FORMAT" default:"json" json:"format"`
		AccessLog AccessLog `json:"access_log"`
	}

	AccessLog struct {
		Enabled            bool `envconfig:"ACCESS_LOG_ENABLED" default:"true" json:"enabled"`
		LogHealthChecks    bool `envconfig:"ACCESS_LOG_HEALTH_CHECKS" default:"false" json:"log_health_checks"`
		IncludeQueryParams bool `envconfig:"ACCESS_LOG_INCLUDE_QUERY_PARAMS" default:"true" json:"include_query_params"`
	}

	Telemetry struct {
		Enabled      bool   `envconfig:"OTEL_ENABLED" default:"false" json:"enabled"`
		ExporterType string `envconfig:"OTEL_EXPORTER" default:"grpc" json:"exporter_type"`

		OTLPEndpoint string `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:"" json:"otlp_endpoint"`

		OtelGRPCHost       string `envconfig:"OTEL_HOST" json:"otel_grpc_host"`
		OtelGRPCPort       string `envconfig:"OTEL_PORT" default:"4317" json:"otel_grpc_port"`
		OtelProductCluster string `envconfig:"OTEL_PRODUCT_CLUSTER" json:"otel_product_cluster"`

		Metrics Metrics `json:"metrics"`
		Traces  Traces  `json:"traces"`
	}

	Metrics struct {
		Enabled bool `envconfig:"METRICS_ENABLED" default:"false" json:"enabled"`
	}

	Traces struct {
		Enabled      bool    `envconfig:"TRACES_ENABLED" default:"false" json:"enabled"`
		SamplerRatio float64 `envconfig:"TRACES_SAMPLER_RATIO" default:"1.0" json:"sampler_ratio"`
	}
)

func (c *ServiceConfig) GetEnvironment() int {
	switch c.App.Env.Name {
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
