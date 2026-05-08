//go:build docker

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const (
	proxyHTTPAddr  = "http://127.0.0.1:4318"
	proxyGRPCAddr  = "127.0.0.1:4317"
	proxyAPIAddr   = "http://127.0.0.1:8080"
	backendAPIAddr = "http://127.0.0.1:14418"
)

func TestMain(m *testing.M) {
	composeFile := "docker-compose.test.yaml"

	fmt.Println("Building Docker Compose test environment...")
	upCmd := exec.Command("docker", "compose", "-f", composeFile, "up", "--build", "-d", "--wait")
	upCmd.Dir = "."
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docker compose up failed: %v\n", err)
		downCmd := exec.Command("docker", "compose", "-f", composeFile, "down", "-v")
		downCmd.Dir = "."
		downCmd.Run()
		os.Exit(1)
	}

	code := m.Run()

	fmt.Println("Tearing down Docker Compose test environment...")
	downCmd := exec.Command("docker", "compose", "-f", composeFile, "down", "-v")
	downCmd.Dir = "."
	downCmd.Stdout = os.Stdout
	downCmd.Stderr = os.Stderr
	downCmd.Run()

	os.Exit(code)
}

func TestDockerHTTPMetricsForwarding(t *testing.T) {
	metrics := pmetric.NewMetrics()
	metricSlice := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	metricSlice.AppendEmpty().SetName("test.metric")
	metricSlice.At(0).SetEmptyGauge()
	metricSlice.At(0).Gauge().DataPoints().AppendEmpty().SetIntValue(42)

	protoBytes, err := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(proxyHTTPAddr+"/v1/metrics", "application/x-protobuf", bytes.NewReader(protoBytes))
	if err != nil {
		t.Fatalf("POST metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics status = %d, want 200", resp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	if !verifyBackendReceived(t, "metrics", 1) {
		t.Error("backend did not receive forwarded metrics")
	}
}

func TestDockerHTTPTracesForwarding(t *testing.T) {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetSpanID([8]byte{1, 2, 3, 4})

	protoBytes, err := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(proxyHTTPAddr+"/v1/traces", "application/x-protobuf", bytes.NewReader(protoBytes))
	if err != nil {
		t.Fatalf("POST traces: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("traces status = %d, want 200", resp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	if !verifyBackendReceived(t, "traces", 1) {
		t.Error("backend did not receive forwarded traces")
	}
}

func TestDockerHTTPLogsForwarding(t *testing.T) {
	logs := plog.NewLogs()
	logRecord := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log message")
	logRecord.SetSeverityText("INFO")

	protoBytes, err := plogotlp.NewExportRequestFromLogs(logs).MarshalProto()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(proxyHTTPAddr+"/v1/logs", "application/x-protobuf", bytes.NewReader(protoBytes))
	if err != nil {
		t.Fatalf("POST logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("logs status = %d, want 200", resp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	if !verifyBackendReceived(t, "logs", 1) {
		t.Error("backend did not receive forwarded logs")
	}
}

func TestDockerProxyHealthz(t *testing.T) {
	resp, err := http.Get(proxyAPIAddr + "/healthz")
	if err != nil {
		t.Fatalf("GET healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "alive" {
		t.Errorf("healthz status = %q, want alive", body["status"])
	}
}

func TestDockerProxyReadyz(t *testing.T) {
	resp, err := http.Get(proxyAPIAddr + "/readyz")
	if err != nil {
		t.Fatalf("GET readyz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("readyz status = %d, want 200, body: %s", resp.StatusCode, string(body))
	}
}

func TestDockerProxyMetricsEndpoint(t *testing.T) {
	resp, err := http.Get(proxyAPIAddr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/metrics status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	output := string(body)
	if len(output) == 0 {
		t.Error("expected non-empty /metrics response")
	}
}

func TestDockerProxyDictionaryAfterSignal(t *testing.T) {
	metrics := pmetric.NewMetrics()
	metricSlice := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	m := metricSlice.AppendEmpty()
	m.SetName("docker.test.metric")
	dp := m.SetEmptyGauge()
	dp.DataPoints().AppendEmpty().SetIntValue(1)
	metricSlice.At(0).Gauge().DataPoints().At(0).Attributes().PutStr("docker.test.attr", "value")

	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	resp, err := http.Post(proxyHTTPAddr+"/v1/metrics", "application/x-protobuf", bytes.NewReader(protoBytes))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	time.Sleep(3 * time.Second)

	dictResp, err := http.Get(proxyAPIAddr + "/api/v1/dictionary")
	if err != nil {
		t.Fatalf("GET dictionary: %v", err)
	}
	defer dictResp.Body.Close()

	if dictResp.StatusCode != http.StatusOK {
		t.Errorf("dictionary status = %d, want 200", dictResp.StatusCode)
	}

	body, _ := io.ReadAll(dictResp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty dictionary response")
	}
}

func TestDockerProxyWeaverExport(t *testing.T) {
	resp, err := http.Get(proxyAPIAddr + "/api/v1/export?format=weaver")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("export status = %d, want 200", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/yaml" {
		t.Errorf("content-type = %q, want text/yaml", ct)
	}
}

func verifyBackendReceived(t *testing.T, signalType string, minCount int64) bool {
	t.Helper()
	resp, err := http.Get(backendAPIAddr + "/stats")
	if err != nil {
		t.Logf("GET backend stats: %v", err)
		return false
	}
	defer resp.Body.Close()

	var stats struct {
		Metrics int64 `json:"metrics"`
		Traces  int64 `json:"traces"`
		Logs    int64 `json:"logs"`
	}
	json.NewDecoder(resp.Body).Decode(&stats)

	var count int64
	switch signalType {
	case "metrics":
		count = stats.Metrics
	case "traces":
		count = stats.Traces
	case "logs":
		count = stats.Logs
	}

	if count < minCount {
		t.Logf("backend %s count = %d, want >= %d", signalType, count, minCount)
		return false
	}
	return true
}
