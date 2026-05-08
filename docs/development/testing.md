# Testing

## Unit Tests

```bash
make test
```

Runs all unit tests with race detection and coverage.

## Integration Tests

```bash
make integration-test
```

Runs integration tests that require local tooling.

## Docker Integration Tests

```bash
make docker-integration-test
```

Full end-to-end tests using Docker Compose with a mock backend.

## Coverage

```bash
make coverage
```

Generates `coverage.txt` and `coverage.html`.

## Writing Tests

Follow table-driven test patterns:

```go
func TestMyComponent(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "test",
            want:  "test",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Coverage Target

Minimum 80% code coverage across all packages.
