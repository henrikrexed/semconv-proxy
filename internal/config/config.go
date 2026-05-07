package config

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

type Config struct {
	OTLPHTTPPort    int    `mapstructure:"otlp-http-port"`
	OTLPGRPCPort    int    `mapstructure:"otlp-grpc-port"`
	APIPort         int    `mapstructure:"api-port"`
	BackendEndpoint string `mapstructure:"backend-endpoint"`
	LogLevel        string `mapstructure:"log-level"`
	ConfigFile      string `mapstructure:"config"`
	DataDir         string `mapstructure:"data-dir"`

	RingBufferSize   int           `mapstructure:"ring-buffer-size"`
	WorkerCount      int           `mapstructure:"worker-count"`
	ShardCount       int           `mapstructure:"shard-count"`
	GlobalBudget     int           `mapstructure:"global-budget"`
	PerAttrCap       int           `mapstructure:"per-attr-cap"`
	TTLStale         time.Duration `mapstructure:"ttl-stale"`
	TTLPurge         time.Duration `mapstructure:"ttl-purge"`
	PersistInterval  time.Duration `mapstructure:"persist-interval"`
	PersistBatchSize int           `mapstructure:"persist-batch-size"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown-timeout"`

	BackendInsecure bool `mapstructure:"backend-insecure"`
}

func Default() *Config {
	return &Config{
		OTLPHTTPPort:     4318,
		OTLPGRPCPort:     4317,
		APIPort:          8080,
		LogLevel:         "info",
		DataDir:          "./data",
		RingBufferSize:   10000,
		WorkerCount:      runtime.NumCPU(),
		ShardCount:       64,
		GlobalBudget:     10000,
		PerAttrCap:       1000,
		TTLStale:         24 * time.Hour,
		TTLPurge:         7 * 24 * time.Hour,
		PersistInterval:  100 * time.Millisecond,
		PersistBatchSize: 1000,
		ShutdownTimeout:  30 * time.Second,
		BackendInsecure:  true,
	}
}

func (c *Config) Validate() error {
	if c.BackendEndpoint == "" {
		return fmt.Errorf("config: backend-endpoint is required")
	}
	if c.OTLPHTTPPort <= 0 || c.OTLPHTTPPort > 65535 {
		return fmt.Errorf("config: otlp-http-port must be between 1 and 65535, got %d", c.OTLPHTTPPort)
	}
	if c.OTLPGRPCPort <= 0 || c.OTLPGRPCPort > 65535 {
		return fmt.Errorf("config: otlp-grpc-port must be between 1 and 65535, got %d", c.OTLPGRPCPort)
	}
	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("config: api-port must be between 1 and 65535, got %d", c.APIPort)
	}
	if c.RingBufferSize <= 0 {
		return fmt.Errorf("config: ring-buffer-size must be positive, got %d", c.RingBufferSize)
	}
	if c.ShardCount <= 0 {
		return fmt.Errorf("config: shard-count must be positive, got %d", c.ShardCount)
	}
	if c.GlobalBudget <= 0 {
		return fmt.Errorf("config: global-budget must be positive, got %d", c.GlobalBudget)
	}
	if c.PerAttrCap <= 0 {
		return fmt.Errorf("config: per-attr-cap must be positive, got %d", c.PerAttrCap)
	}
	return nil
}

func (c *Config) OTLPHTTPAddr() string {
	return fmt.Sprintf(":%d", c.OTLPHTTPPort)
}

func (c *Config) OTLPGRPCAddr() string {
	return fmt.Sprintf(":%d", c.OTLPGRPCPort)
}

func (c *Config) APIAddr() string {
	return fmt.Sprintf(":%d", c.APIPort)
}

func (c *Config) Environ() map[string]string {
	return map[string]string{
		"SEMCONV_PROXY_BACKEND_ENDPOINT": c.BackendEndpoint,
		"SEMCONV_PROXY_OTLP_HTTP_PORT":   fmt.Sprintf("%d", c.OTLPHTTPPort),
		"SEMCONV_PROXY_OTLP_GRPC_PORT":   fmt.Sprintf("%d", c.OTLPGRPCPort),
		"SEMCONV_PROXY_API_PORT":         fmt.Sprintf("%d", c.APIPort),
		"SEMCONV_PROXY_LOG_LEVEL":        c.LogLevel,
		"SEMCONV_PROXY_DATA_DIR":         c.DataDir,
	}
}

func EnsureDataDir(path string) error {
	if path == "" {
		path = "./data"
	}
	return os.MkdirAll(path, 0755)
}
