# Docker Deployment

## Running the Container

The proxy image is published to GitHub Container Registry:

```bash
docker pull ghcr.io/henrikrexed/semconv-proxy:latest
```

Run with all default ports:

```bash
docker run -d \
  --name semconv-proxy \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8080:8080 \
  -v proxy-data:/data \
  ghcr.io/henrikrexed/semconv-proxy:latest \
  --backend-endpoint=otel-collector:4317
```

### Port Mapping

| Port | Protocol | Purpose |
|------|----------|---------|
| 4317 | gRPC | OTLP signal ingestion |
| 4318 | HTTP | OTLP signal ingestion |
| 8080 | HTTP | REST API, health probes, Prometheus metrics |

### Persistence

Mount a volume for Pebble storage to survive container restarts:

```bash
docker run -d \
  -v semconv-proxy-data:/data \
  -p 4317:4317 -p 4318:4318 -p 8080:8080 \
  ghcr.io/henrikrexed/semconv-proxy:latest \
  --backend-endpoint=otel-collector:4317 \
  --data-dir=/data
```

## Docker Compose

The repository includes a Docker Compose file at `deployments/docker/docker-compose.yaml`:

```yaml
version: "3.8"

services:
  proxy:
    build: .
    ports:
      - "4317:4317"
      - "4318:4318"
      - "8080:8080"
    command:
      - --backend-endpoint=otel-collector:4317
      - --log-level=debug
    volumes:
      - proxy-data:/data

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: --config=/etc/otel-collector-config.yaml
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml

volumes:
  proxy-data:
```

Start the full stack:

```bash
cd deployments/docker
docker compose up -d
```

Verify it's running:

```bash
curl http://localhost:8080/healthz
```

## Building the Image

Build from source:

```bash
docker build -t semconv-proxy .
```

The Dockerfile uses a multi-stage build:

1. **Builder stage** — compiles the Go binary with CGO disabled
2. **Runtime stage** — distroless non-root image (<50MB)

The image supports `linux/amd64` and `linux/arm64` via buildx:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/henrikrexed/semconv-proxy:latest .
```

## Resource Limits

Recommended Docker resource limits:

```bash
docker run -d \
  --memory=512m \
  --cpus=0.5 \
  -p 4317:4317 -p 4318:4318 -p 8080:8080 \
  ghcr.io/henrikrexed/semconv-proxy:latest \
  --backend-endpoint=otel-collector:4317
```

## Environment Variables

All configuration settings are available as environment variables:

```bash
docker run -d \
  -e SEMCONV_PROXY_BACKEND_ENDPOINT=otel-collector:4317 \
  -e SEMCONV_PROXY_LOG_LEVEL=debug \
  -e SEMCONV_PROXY_OTLP_HTTP_PORT=4318 \
  -e SEMCONV_PROXY_OTLP_GRPC_PORT=4317 \
  -e SEMCONV_PROXY_API_PORT=8080 \
  -p 4317:4317 -p 4318:4318 -p 8080:8080 \
  ghcr.io/henrikrexed/semconv-proxy:latest
```
