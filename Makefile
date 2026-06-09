.PHONY: build test lint integration-test docker coverage clean run \
       security-scan vulncheck gosec trivy-fs sbom docs docs-serve \
       release snapshot docker-push multiplatform-build \
       semconv-registry semconv-registry-check

BINARY := semconv-proxy
CMD := ./cmd/semconv-proxy
GO := go
GOLANGCI_LINT := golangci-lint
DOCKER := docker
SYFT := syft
TRIVY := trivy
GOVULNCHECK := govulncheck
GOSEC := gosec
MKDOCS := mkdocs

build:
	$(GO) build -o $(BINARY) $(CMD)

multiplatform-build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY)-linux-amd64 $(CMD)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY)-linux-arm64 $(CMD)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY)-darwin-arm64 $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(BINARY)-windows-amd64.exe $(CMD)

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

docker-push:
	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/henrikrexed/semconv-proxy:latest --push .

helm-package:
	helm package deployments/helm/semconv-proxy/

# Regenerate the embedded semconv snapshot (internal/semconv/data/registry.json)
# from the pinned semconv release using the pinned Weaver binary. Run after a
# version bump, then commit the result. See docs/development/semconv-registry.md.
semconv-registry:
	./scripts/resolve-registry.sh

# CI guard: fail if the committed snapshot drifts from a fresh pinned resolve.
semconv-registry-check:
	./scripts/resolve-registry.sh --check

security-scan: vulncheck gosec trivy-fs

vulncheck:
	$(GOVULNCHECK) ./...

gosec:
	$(GOSEC) -no-fail -fmt sarif -out gosec-results.sarif ./...

trivy-fs:
	$(TRIVY) fs --severity CRITICAL,HIGH --format table .

sbom:
	$(SYFT) dir:./ -o spdx-json > sbom-source.spdx.json

release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean

docs:
	$(MKDOCS) build -d site

docs-serve:
	$(MKDOCS) serve

clean:
	rm -f $(BINARY) $(BINARY)-linux-* $(BINARY)-darwin-* $(BINARY)-windows-*.exe
	rm -f coverage.txt coverage.html gosec-results.sarif sbom-source.spdx.json
	rm -rf site/
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
