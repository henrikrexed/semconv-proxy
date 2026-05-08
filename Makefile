.PHONY: build test lint integration-test docker coverage clean run

BINARY := semconv-proxy
CMD := ./cmd/semconv-proxy
GO := go
GOLANGCI_LINT := golangci-lint
DOCKER := docker

build:
	$(GO) build -o $(BINARY) $(CMD)

test:
	$(GO) test -race -count=1 ./...

integration-test:
	$(GO) test -race -tags=integration -count=1 ./tests/integration/...

docker-integration-test:
	$(GO) test -race -tags=docker -count=1 -timeout 300s ./tests/integration/...

lint:
	$(GOLANGCI_LINT) run ./...

coverage:
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic ./...
	$(GO) tool cover -html=coverage.txt -o coverage.html

docker:
	$(DOCKER) build -t semconv-proxy:latest .

docker-buildx:
	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 -t semconv-proxy:latest .

helm-package:
	helm package deployments/helm/semconv-proxy/

clean:
	rm -f $(BINARY) coverage.txt coverage.html
	$(GO) clean ./...

run: build
	./$(BINARY) --backend-endpoint=localhost:4317 --log-level=debug

tidy:
	$(GO) mod tidy

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .
	goimports -w .
