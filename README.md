# Collector Semantic Convention Proxy

A standalone Go OTLP proxy that sits between OpenTelemetry Collectors and observability backends. It auto-discovers semantic conventions from live telemetry signals (metrics, traces, logs), builds a live dictionary, and exports in OTel Weaver-compatible YAML format.

## Quick Start

```bash
# Build
make build

# Run
./semconv-proxy --backend-endpoint=localhost:4317

# Run with Docker
docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 semconv-proxy --backend-endpoint=otel-collector:4317
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--backend-endpoint` | `SEMCONV_PROXY_BACKEND_ENDPOINT` | required | OTLP backend endpoint |
| `--otlp-http-port` | `SEMCONV_PROXY_OTLP_HTTP_PORT` | 4318 | OTLP/HTTP listen port |
| `--otlp-grpc-port` | `SEMCONV_PROXY_OTLP_GRPC_PORT` | 4317 | OTLP/gRPC listen port |
| `--api-port` | `SEMCONV_PROXY_API_PORT` | 8080 | REST API listen port |
| `--log-level` | `SEMCONV_PROXY_LOG_LEVEL` | info | Log level (debug/info/warn/error) |
| `--config` | | | Path to YAML config file |

## Development

```bash
make build          # Build binary
make test           # Run unit tests
make lint           # Run golangci-lint
make docker         # Build Docker image
```

## License

Apache 2.0
