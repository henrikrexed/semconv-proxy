package config

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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

// ApplyFile reads the YAML config file at path and overlays its values onto cfg.
// File keys may be snake_case (as emitted by the Helm ConfigMap, e.g. log_level)
// or hyphenated (matching the CLI flags, e.g. log-level); both are normalized to
// the struct's mapstructure tags before decoding. Keys absent from the file keep
// their existing value, so default-supplied fields are preserved. It is the
// lowest-priority overlay above defaults; callers apply ApplyEnv and ApplyFlags
// afterwards so env and explicit flags win, per docs/getting-started/configuration.md.
func ApplyFile(cfg *Config, path string) error {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	for _, key := range v.AllKeys() {
		if norm := strings.ReplaceAll(key, "_", "-"); norm != key {
			v.Set(norm, v.Get(key))
		}
	}
	return v.Unmarshal(cfg)
}

// envKeys maps the SEMCONV_PROXY_* environment variables to the Config
// mapstructure keys they override. Only the connection/runtime knobs that the
// Helm chart and 12-factor deployments expose as env are wired here, matching
// the table in docs/getting-started/configuration.md.
var envKeys = map[string]string{
	"SEMCONV_PROXY_BACKEND_ENDPOINT": "backend-endpoint",
	"SEMCONV_PROXY_OTLP_HTTP_PORT":   "otlp-http-port",
	"SEMCONV_PROXY_OTLP_GRPC_PORT":   "otlp-grpc-port",
	"SEMCONV_PROXY_API_PORT":         "api-port",
	"SEMCONV_PROXY_LOG_LEVEL":        "log-level",
	"SEMCONV_PROXY_DATA_DIR":         "data-dir",
}

// ApplyEnv overlays any set SEMCONV_PROXY_* environment variables onto cfg.
// Unset variables are skipped so they never clobber a config-file or default
// value. Env takes precedence over the config file but is overridden by
// explicit CLI flags (see ApplyFlags).
func ApplyEnv(cfg *Config) error {
	v := viper.New()
	set := false
	for env, key := range envKeys {
		if val, ok := os.LookupEnv(env); ok {
			v.Set(key, val)
			set = true
		}
	}
	if !set {
		return nil
	}
	return v.Unmarshal(cfg)
}

// ApplyFlags overlays explicitly-set CLI flags onto cfg, the highest-priority
// source. Flags left at their default are not visited and so never clobber a
// value supplied by the config file or environment.
func ApplyFlags(cfg *Config, flags *pflag.FlagSet) error {
	v := viper.New()
	set := false
	flags.Visit(func(f *pflag.Flag) {
		v.Set(f.Name, f.Value.String())
		set = true
	})
	if !set {
		return nil
	}
	return v.Unmarshal(cfg)
}

func EnsureDataDir(path string) error {
	if path == "" {
		path = "./data"
	}
	return os.MkdirAll(path, 0755)
}
