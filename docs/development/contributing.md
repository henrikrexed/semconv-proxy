# Contributing

## Prerequisites

- Go 1.23+
- golangci-lint
- Docker (for integration tests)

## Development Setup

```bash
git clone https://github.com/henrikrexed/semconv-proxy.git
cd semconv-proxy
make build
make test
make lint
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `make lint` and `make test`
5. Submit a pull request

## Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` and `goimports`
- All public types must have doc comments
- Table-driven tests for new test cases
