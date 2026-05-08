package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
)

type receivedSignal struct {
	Path string
	Size int
}

var (
	mu          sync.Mutex
	received    []receivedSignal
	metricCount atomic.Int64
	traceCount  atomic.Int64
	logCount    atomic.Int64
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/metrics", handleSignal("metrics", &metricCount))
	mux.HandleFunc("/v1/traces", handleSignal("traces", &traceCount))
	mux.HandleFunc("/v1/logs", handleSignal("logs", &logCount))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"alive"}`)
	})
	mux.HandleFunc("/stats", handleStats)

	fmt.Println("mock-backend listening on :4318")
	if err := http.ListenAndServe(":4318", mux); err != nil {
		fmt.Printf("mock-backend error: %v\n", err)
	}
}

func handleSignal(name string, counter *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		counter.Add(1)
		mu.Lock()
		received = append(received, receivedSignal{Path: r.URL.Path, Size: len(body)})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{0x0a, 0x00})
	}
}

func handleStats(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"metrics":%d,"traces":%d,"logs":%d}`, metricCount.Load(), traceCount.Load(), logCount.Load())
}
