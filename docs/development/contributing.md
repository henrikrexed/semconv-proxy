# Contributing

## Prerequisites

- Go 1.23+
- Docker (for integration tests)
- `golangci-lint` (for linting)

## Setup

```bash
git clone https://github.com/henrikrexed/semconv-proxy.git
cd semconv-proxy
go mod download
make build
```

## Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes
4. Run tests: `make test`
5. Run lint: `make lint`
6. Commit with a descriptive message
7. Push and open a Pull Request

## Code Style

- Follow standard Go conventions (`gofmt`, `goimports`)
- All code lives under `internal/` — no public Go API in MVP
- Use `log/slog` for all logging — never `fmt.Println`
- Use `context.Context` as the first parameter in cancellable functions
- Return `error` as the last return value
- Wrap errors with `fmt.Errorf("package: operation: %w", err)`
- No panics in production code

## Project Structure

```
internal/
├── config/       Configuration (Cobra + Viper)
├── receiver/     OTLP signal receivers
├── exporter/     OTLP signal forwarder
├── analysis/     Ring buffer + worker pool + extractor
├── dictionary/   Sharded in-memory dictionary
├── storage/      Pebble persistence
├── cardinality/  Cardinality tracking
├── api/          REST API
├── export/       Weaver YAML export
├── health/       Health aggregation
├── metrics/      Prometheus metrics
├── lifecycle/    Graceful shutdown
└── testutil/     Test helpers
```

## Pull Request Checklist

- [ ] Tests pass: `make test`
- [ ] Lint passes: `make lint`
- [ ] New code has tests
- [ ] No new dependencies without justification
- [ ] Commit messages are descriptive

## License

Apache 2.0 — see [LICENSE](https://github.com/henrikrexed/semconv-proxy/blob/main/LICENSE).
