# Installation

## Pre-built Binaries

Download from [GitHub Releases](https://github.com/henrikrexed/semconv-proxy/releases):

| Platform | Architecture | Binary |
|----------|-------------|--------|
| Linux | amd64 | `semconv-proxy_linux_amd64.tar.gz` |
| Linux | arm64 | `semconv-proxy_linux_arm64.tar.gz` |
| macOS | amd64 | `semconv-proxy_darwin_amd64.tar.gz` |
| macOS | arm64 | `semconv-proxy_darwin_arm64.tar.gz` |
| Windows | amd64 | `semconv-proxy_windows_amd64.zip` |

```bash
# Example: Linux amd64
tar xzf semconv-proxy_0.1.0_linux_amd64.tar.gz
chmod +x semconv-proxy
sudo mv semconv-proxy /usr/local/bin/
```

## Docker

```bash
docker pull ghcr.io/henrikrexed/semconv-proxy:latest
```

Multi-platform images are available for `linux/amd64` and `linux/arm64`.

## Helm Chart (Kubernetes)

From the published OCI chart on GHCR:

```bash
helm install semconv-proxy \
  oci://ghcr.io/henrikrexed/semconv-proxy-chart \
  --version 0.1.0 \
  --set config.backendEndpoint=otel-collector.observability:4317
```

Or from the in-tree source:

```bash
helm install semconv-proxy ./deployments/helm/semconv-proxy-chart \
  --set config.backendEndpoint=otel-collector.observability:4317
```

## Build from Source

```bash
git clone https://github.com/henrikrexed/semconv-proxy.git
cd semconv-proxy
make build
```

## Verify Installation

```bash
semconv-proxy --version
```
