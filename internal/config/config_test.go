package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// Regression for S0-1/S1-3/S1-4: the Helm ConfigMap emits snake_case keys; every
// value (not just the three passed as flags) must reach cfg.
func TestApplyFile_SnakeCaseKeys(t *testing.T) {
	path := writeConfig(t, `
backend_endpoint: backend:4317
backend_insecure: false
log_level: debug
data_dir: /srv/data
otlp_http_port: 5318
otlp_grpc_port: 5317
api_port: 9090
shard_count: 16
global_budget: 500
per_attr_cap: 50
ring_buffer_size: 2048
worker_count: 3
ttl_stale: 48h
ttl_purge: 96h
`)
	cfg := Default()
	if err := ApplyFile(cfg, path); err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"BackendEndpoint", cfg.BackendEndpoint, "backend:4317"},
		{"BackendInsecure", cfg.BackendInsecure, false},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"DataDir", cfg.DataDir, "/srv/data"},
		{"OTLPHTTPPort", cfg.OTLPHTTPPort, 5318},
		{"OTLPGRPCPort", cfg.OTLPGRPCPort, 5317},
		{"APIPort", cfg.APIPort, 9090},
		{"ShardCount", cfg.ShardCount, 16},
		{"GlobalBudget", cfg.GlobalBudget, 500},
		{"PerAttrCap", cfg.PerAttrCap, 50},
		{"RingBufferSize", cfg.RingBufferSize, 2048},
		{"WorkerCount", cfg.WorkerCount, 3},
		{"TTLStale", cfg.TTLStale, 48 * time.Hour},
		{"TTLPurge", cfg.TTLPurge, 96 * time.Hour},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// The hyphenated form used by CLI flags must decode identically.
func TestApplyFile_HyphenatedKeys(t *testing.T) {
	path := writeConfig(t, "backend-endpoint: be:4317\nlog-level: warn\nttl-stale: 12h\n")
	cfg := Default()
	if err := ApplyFile(cfg, path); err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if cfg.BackendEndpoint != "be:4317" {
		t.Errorf("BackendEndpoint = %q, want be:4317", cfg.BackendEndpoint)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", cfg.LogLevel)
	}
	if cfg.TTLStale != 12*time.Hour {
		t.Errorf("TTLStale = %v, want 12h", cfg.TTLStale)
	}
}

// Keys absent from the file must retain their prior (flag/default) value.
func TestApplyFile_PartialOverlayPreservesDefaults(t *testing.T) {
	path := writeConfig(t, "log_level: error\n")
	cfg := Default()
	cfg.BackendEndpoint = "preset:4317"
	if err := ApplyFile(cfg, path); err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error", cfg.LogLevel)
	}
	if cfg.BackendEndpoint != "preset:4317" {
		t.Errorf("BackendEndpoint = %q, want preserved preset:4317", cfg.BackendEndpoint)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("APIPort = %d, want preserved default 8080", cfg.APIPort)
	}
}

// S1-3: backend_insecure:false must actually disable insecure mode, not silently no-op.
func TestApplyFile_BackendInsecureFalse(t *testing.T) {
	path := writeConfig(t, "backend_insecure: false\n")
	cfg := Default() // default is true
	if err := ApplyFile(cfg, path); err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if cfg.BackendInsecure {
		t.Error("BackendInsecure = true, want false from file")
	}
}

func TestApplyFile_FileNotFound(t *testing.T) {
	cfg := Default()
	if err := ApplyFile(cfg, filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("ApplyFile() error = nil, want error for missing file")
	}
}

func TestApplyFile_InvalidYAML(t *testing.T) {
	path := writeConfig(t, "log_level: : : bad\n\t- broken")
	cfg := Default()
	if err := ApplyFile(cfg, path); err == nil {
		t.Error("ApplyFile() error = nil, want error for invalid YAML")
	}
}

// Set SEMCONV_PROXY_* vars must reach cfg, including string->int/duration coercion.
func TestApplyEnv_SetVarsOverlay(t *testing.T) {
	t.Setenv("SEMCONV_PROXY_BACKEND_ENDPOINT", "env-be:4317")
	t.Setenv("SEMCONV_PROXY_API_PORT", "7070")
	t.Setenv("SEMCONV_PROXY_LOG_LEVEL", "warn")
	cfg := Default()
	if err := ApplyEnv(cfg); err != nil {
		t.Fatalf("ApplyEnv: %v", err)
	}
	if cfg.BackendEndpoint != "env-be:4317" {
		t.Errorf("BackendEndpoint = %q, want env-be:4317", cfg.BackendEndpoint)
	}
	if cfg.APIPort != 7070 {
		t.Errorf("APIPort = %d, want 7070", cfg.APIPort)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", cfg.LogLevel)
	}
}

// Unset env vars must leave existing flag/file/default values untouched.
func TestApplyEnv_UnsetVarsPreserve(t *testing.T) {
	cfg := Default()
	cfg.BackendEndpoint = "preset:4317"
	cfg.APIPort = 9999
	if err := ApplyEnv(cfg); err != nil {
		t.Fatalf("ApplyEnv: %v", err)
	}
	if cfg.BackendEndpoint != "preset:4317" {
		t.Errorf("BackendEndpoint = %q, want preserved preset:4317", cfg.BackendEndpoint)
	}
	if cfg.APIPort != 9999 {
		t.Errorf("APIPort = %d, want preserved 9999", cfg.APIPort)
	}
}

func testFlags(cfg *Config) *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("backend-endpoint", cfg.BackendEndpoint, "")
	fs.Int("api-port", cfg.APIPort, "")
	fs.String("log-level", cfg.LogLevel, "")
	return fs
}

// Only explicitly-set flags overlay; defaulted flags must not clobber cfg.
func TestApplyFlags_OnlyChangedOverlay(t *testing.T) {
	cfg := Default()
	cfg.BackendEndpoint = "file:4317"
	cfg.APIPort = 8081
	fs := testFlags(cfg)
	if err := fs.Parse([]string{"--log-level=error"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := ApplyFlags(cfg, fs); err != nil {
		t.Fatalf("ApplyFlags: %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error from flag", cfg.LogLevel)
	}
	if cfg.BackendEndpoint != "file:4317" {
		t.Errorf("BackendEndpoint = %q, want preserved file:4317", cfg.BackendEndpoint)
	}
	if cfg.APIPort != 8081 {
		t.Errorf("APIPort = %d, want preserved 8081", cfg.APIPort)
	}
}

// Full precedence chain: default < file < env < explicit flag (configuration.md).
func TestPrecedence_FlagBeatsEnvBeatsFile(t *testing.T) {
	path := writeConfig(t, "log_level: debug\napi_port: 5050\nbackend_endpoint: file:4317\n")
	t.Setenv("SEMCONV_PROXY_LOG_LEVEL", "warn")
	t.Setenv("SEMCONV_PROXY_API_PORT", "6060")

	cfg := Default()
	if err := ApplyFile(cfg, path); err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if err := ApplyEnv(cfg); err != nil {
		t.Fatalf("ApplyEnv: %v", err)
	}
	fs := testFlags(cfg)
	if err := fs.Parse([]string{"--log-level=error"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := ApplyFlags(cfg, fs); err != nil {
		t.Fatalf("ApplyFlags: %v", err)
	}

	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error (flag beats env+file)", cfg.LogLevel)
	}
	if cfg.APIPort != 6060 {
		t.Errorf("APIPort = %d, want 6060 (env beats file)", cfg.APIPort)
	}
	if cfg.BackendEndpoint != "file:4317" {
		t.Errorf("BackendEndpoint = %q, want file:4317 (file beats default)", cfg.BackendEndpoint)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.OTLPHTTPPort != 4318 {
		t.Errorf("Default OTLPHTTPPort = %d, want 4318", cfg.OTLPHTTPPort)
	}
	if cfg.OTLPGRPCPort != 4317 {
		t.Errorf("Default OTLPGRPCPort = %d, want 4317", cfg.OTLPGRPCPort)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("Default APIPort = %d, want 8080", cfg.APIPort)
	}
	if cfg.ShardCount != 64 {
		t.Errorf("Default ShardCount = %d, want 64", cfg.ShardCount)
	}
	if cfg.GlobalBudget != 10000 {
		t.Errorf("Default GlobalBudget = %d, want 10000", cfg.GlobalBudget)
	}
	if cfg.PerAttrCap != 1000 {
		t.Errorf("Default PerAttrCap = %d, want 1000", cfg.PerAttrCap)
	}
	if cfg.RingBufferSize != 10000 {
		t.Errorf("Default RingBufferSize = %d, want 10000", cfg.RingBufferSize)
	}
	if cfg.TTLStale != 24*time.Hour {
		t.Errorf("Default TTLStale = %v, want 24h", cfg.TTLStale)
	}
	if cfg.TTLPurge != 7*24*time.Hour {
		t.Errorf("Default TTLPurge = %v, want 168h", cfg.TTLPurge)
	}
	if cfg.BackendEndpoint != "" {
		t.Errorf("Default BackendEndpoint = %q, want empty", cfg.BackendEndpoint)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317" },
			wantErr: false,
		},
		{
			name:    "missing backend endpoint",
			modify:  func(c *Config) { c.BackendEndpoint = "" },
			wantErr: true,
			errMsg:  "backend-endpoint is required",
		},
		{
			name:    "invalid http port zero",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.OTLPHTTPPort = 0 },
			wantErr: true,
			errMsg:  "otlp-http-port",
		},
		{
			name:    "invalid http port too high",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.OTLPHTTPPort = 70000 },
			wantErr: true,
			errMsg:  "otlp-http-port",
		},
		{
			name:    "invalid grpc port",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.OTLPGRPCPort = -1 },
			wantErr: true,
			errMsg:  "otlp-grpc-port",
		},
		{
			name:    "invalid api port",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.APIPort = 0 },
			wantErr: true,
			errMsg:  "api-port",
		},
		{
			name:    "invalid ring buffer size",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.RingBufferSize = -1 },
			wantErr: true,
			errMsg:  "ring-buffer-size",
		},
		{
			name:    "invalid shard count",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.ShardCount = 0 },
			wantErr: true,
			errMsg:  "shard-count",
		},
		{
			name:    "invalid global budget",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.GlobalBudget = -1 },
			wantErr: true,
			errMsg:  "global-budget",
		},
		{
			name:    "invalid per attr cap",
			modify:  func(c *Config) { c.BackendEndpoint = "localhost:4317"; c.PerAttrCap = 0 },
			wantErr: true,
			errMsg:  "per-attr-cap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestAddrFormats(t *testing.T) {
	cfg := &Config{OTLPHTTPPort: 4318, OTLPGRPCPort: 4317, APIPort: 8080}
	if cfg.OTLPHTTPAddr() != ":4318" {
		t.Errorf("OTLPHTTPAddr() = %q, want :4318", cfg.OTLPHTTPAddr())
	}
	if cfg.OTLPGRPCAddr() != ":4317" {
		t.Errorf("OTLPGRPCAddr() = %q, want :4317", cfg.OTLPGRPCAddr())
	}
	if cfg.APIAddr() != ":8080" {
		t.Errorf("APIAddr() = %q, want :8080", cfg.APIAddr())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
