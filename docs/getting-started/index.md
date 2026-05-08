# Getting Started

Get Semconv Proxy up and running in minutes.

## Prerequisites

- Go 1.23+ (for building from source)
- Docker (for containerized deployment)
- An OpenTelemetry Collector or SDK sending OTLP data

## Installation Options

### Binary (Pre-built)

Download the latest release from [GitHub Releases](https://github.com/henrikrexed/semconv-proxy/releases).

### Docker

```bash
docker pull ghcr.io/henrikrexed/semconv-proxy:latest
docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 \
  ghcr.io/henrikrexed/semconv-proxy:latest \
  --backend-endpoint=otel-collector:4317
```

### Build from Source

```bash
git clone https://github.com/henrikrexed/semconv-proxy.git
cd semconv-proxy
make build
./semconv-proxy --backend-endpoint=localhost:4317
```

## Next Steps

- [Quick Start](quick-start.md) — Send your first signals through the proxy
- [Configuration](configuration.md) — Customize proxy behavior
