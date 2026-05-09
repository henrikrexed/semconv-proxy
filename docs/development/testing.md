# Testing

## Running Tests

```bash
# Unit tests
make test

# Or directly
go test ./internal/...

# With verbose output
go test -v ./internal/...

# Specific package
go test -v ./internal/dictionary/...

# With coverage
go test -cover ./internal/...

# Integration tests
go test -tags=integration ./tests/integration/...
```

## Test Strategy

### Unit Tests

Co-located with source code. Each package has its own test files:

```
internal/dictionary/
├── dictionary.go
├── dictionary_test.go
├── shard.go
├── shard_test.go
└── entry.go
```

Tests follow the table-driven pattern:

```go
func TestDictionary_Upsert(t *testing.T) {
    tests := []struct {
        name    string
        entry   *AttributeEntry
        wantNew bool
    }{
        {
            name: "new attribute",
            entry: &AttributeEntry{Name: "http.method", Type: "string"},
            wantNew: true,
        },
        {
            name: "existing attribute",
            entry: &AttributeEntry{Name: "http.method", Type: "string"},
            wantNew: false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Integration Tests

Located in `tests/integration/` with the `//go:build integration` build tag.

Key integration tests:

| Test | Description |
|------|-------------|
| `proxy_test.go` | End-to-end proxy: send signals, verify forwarding and dictionary |
| `dictionary_test.go` | Dictionary lifecycle: create, populate, query, persist, recover |
| `export_test.go` | Weaver export: generate YAML and validate format |

### Benchmarks

Hot-path code includes benchmark tests:

```bash
# Ring buffer write performance
go test -bench=BenchmarkRingBuffer -benchmem ./internal/analysis/

# Dictionary read performance
go test -bench=BenchmarkDictionary -benchmem ./internal/dictionary/

# Cardinality tracking
go test -bench=BenchmarkTracker -benchmem ./internal/cardinality/
```

Targets:

| Benchmark | Target |
|-----------|--------|
| Ring buffer write | <100ns/op |
| Dictionary read | <1μs/op |
| Pebble batch write | <10ms for 1000 entries |

## Test Utilities

`internal/testutil/` provides shared helpers for constructing test fixtures:

- OTLP signal builders (metrics, traces, logs)
- Dictionary population helpers
- Mock backend server for forwarding tests

## CI Pipeline

The GitHub Actions CI pipeline runs on every PR:

1. `go vet` — static analysis
2. `golangci-lint` — comprehensive linter
3. `go test ./internal/...` — all unit tests
4. `go build ./...` — build verification

Configuration: `.github/workflows/ci.yaml`
