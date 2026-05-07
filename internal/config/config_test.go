package config

import (
	"testing"
	"time"
)

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
