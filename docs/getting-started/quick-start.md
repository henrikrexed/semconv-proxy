# Quick Start

Get SemConv Proxy running and discover your first semantic conventions in under 5 minutes.

## Prerequisites

- Docker and Docker Compose
- `curl` for testing API endpoints

## Step 1: Start the Proxy with Docker Compose

Clone the repository and use the provided Docker Compose file:

```bash
git clone https://github.com/henrikrexed/semconv-proxy.git
cd semconv-proxy
```

The `deployments/docker/docker-compose.yaml` file includes the proxy and an OpenTelemetry Collector:

```yaml
services:
  proxy:
    build: .
    ports:
      - "4317:4317"   # OTLP/gRPC
      - "4318:4318"   # OTLP/HTTP
      - "8080:8080"   # REST API
    command:
      - --backend-endpoint=otel-collector:4317
      - --log-level=debug

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: --config=/etc/otel-collector-config.yaml
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
```

Start everything:

```bash
cd deployments/docker
docker compose up -d
```

## Step 2: Send Telemetry

Send a sample metric through the proxy using the OTLP/HTTP endpoint:

```bash
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/octet-stream" \
  -d @test-metric.pb
```

Or configure your OpenTelemetry SDK to export to `http://localhost:4318` (HTTP) or `localhost:4317` (gRPC).

## Step 3: Query the Dictionary

After sending telemetry, query the live dictionary:

```bash
curl http://localhost:8080/api/v1/dictionary | jq .
```

Response:

```json
{
  "total": 12,
  "offset": 0,
  "limit": 100,
  "entries": [
    {
      "name": "http.request.method",
      "type": "string",
      "signal_types": ["metric", "trace"],
      "cardinality": 6,
      "first_seen": "2026-05-08T10:00:00Z",
      "last_seen": "2026-05-08T15:30:00Z",
      "status": "active"
    }
  ]
}
```

## Step 4: Check Cardinality

View high-cardinality attributes:

```bash
curl http://localhost:8080/api/v1/cardinality | jq .
```

## Step 5: Export Weaver YAML

Export discovered conventions in OTel Weaver format:

```bash
curl "http://localhost:8080/api/v1/export?format=weaver" -o semconv.yaml
```

Validate with Weaver:

```bash
weaver registry check -r semconv.yaml
```

## Step 6: Verify Health

Check proxy health and readiness:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics
```

## What's Next?

- [Configuration](configuration.md) — tune performance and cardinality limits
- [Architecture](../architecture/index.md) — understand the internal design
- [Use Cases](../use-cases/index.md) — real-world deployment patterns
