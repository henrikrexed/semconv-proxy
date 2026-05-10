# Semconv Proxy

[![CI](https://github.com/henrikrexed/semconv-proxy/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/henrikrexed/semconv-proxy/actions/workflows/ci.yaml)

**OpenTelemetry Collector Semantic Convention Proxy** — a transparent proxy that intercepts, analyzes, and forwards OpenTelemetry signals to discover, track, and export semantic conventions in real time.

## Why Semconv Proxy?

As OpenTelemetry adoption grows, organizations struggle to understand which semantic conventions their services actually emit. Existing tools only validate conventions after the fact. Semconv Proxy solves this by sitting transparently in the telemetry pipeline, providing:

- **Zero-loss forwarding** — sub-millisecond p99 overhead on signal throughput
- **Real-time discovery** — automatically catalog every attribute across metrics, traces, and logs
- **Change detection** — track when attributes are added, modified, or removed
- **Weaver integration** — export discovered conventions as Weaver-compatible YAML
- **Self-observability** — 30+ Prometheus metrics for monitoring the proxy itself

## Quick Links

- [Getting Started](getting-started/index.md) — Install and run in 5 minutes
- [Architecture](architecture/index.md) — How it works and pipeline integration
- [Use Cases](use-cases/index.md) — Real-world scenarios and examples
- [API Reference](api/index.md) — REST API and metrics endpoints

## Features

| Feature | Description |
|---------|-------------|
| OTLP Receivers | HTTP (4318) and gRPC (4317) protocol support |
| Zero-Loss Forwarding | Sub-millisecond overhead, backpressure-aware |
| 64-Shard Dictionary | FNV-1a hashed concurrent attribute storage |
| Ring Buffer Analysis | Drop-oldest overflow with configurable worker pool |
| Pebble Persistence | Crash-recoverable write-behind storage |
| Cardinality Tracking | HyperLogLog, Count-Min Sketch, Top-K |
| Change Detection | New, changed, and removed attribute tracking |
| REST API | Query, search, and filter the convention dictionary |
| Weaver Export | Generate Weaver-compatible YAML from live data |
| Prometheus Metrics | 30+ self-observability metrics |
| Multi-Platform | Linux, macOS, Windows binaries + Docker images |
