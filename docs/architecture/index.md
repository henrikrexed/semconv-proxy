# Architecture

SemConv Proxy is a Go-based OTLP proxy that sits transparently between your OpenTelemetry Collectors and your observability backend, auto-discovering semantic conventions from live telemetry.

## Design Principles

- **Zero-loss forwarding** — signals are always forwarded, even when analysis is saturated
- **Async by default** — analysis and persistence never block the data path
- **Bounded memory** — cardinality caps and global budgets prevent unbounded growth
- **Crash-recoverable** — Pebble persistence ensures dictionary state survives restarts

## Where to Learn More

- [Overview](overview.md) — high-level architecture and how the proxy fits into your pipeline
- [Data Flow](data-flow.md) — step-by-step path of a signal through the proxy
- [Components](components.md) — detailed description of each internal component
- [Pipeline Integration](pipeline-integration.md) — deployment patterns for different environments
