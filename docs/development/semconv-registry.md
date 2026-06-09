# Semantic-Convention Registry Snapshot

The proxy ships with a **build-time pinned** snapshot of the official
OpenTelemetry semantic-convention registry at
`internal/semconv/data/registry.json`. It is embedded into the binary with
`go:embed`, so the running proxy has **no runtime dependency on Weaver or network
access** — it powers the [Community SemConv API](../api/community.md) and the
[Web UI](../operations/web-ui.md).

The snapshot is produced by [Weaver](https://github.com/open-telemetry/weaver)'s
`registry resolve` against a pinned semconv release. Weaver runs at build time
only.

## Pinned versions

| Pin | Value | Where |
|-----|-------|-------|
| semconv release | `v1.41.1` | `SEMCONV_VERSION` in `scripts/resolve-registry.sh` |
| Weaver binary | `0.23.0` | `WEAVER_VERSION` in `scripts/resolve-registry.sh` |

The resolve is **reproducible**: a fresh resolve against the same pins is
byte-identical to the committed `registry.json`. CI enforces this with a drift
check (`make semconv-registry-check`) so the snapshot can never silently diverge
from its pinned source.

## Regenerate the snapshot

```bash
make semconv-registry
```

This downloads the pinned Weaver binary (cached under `.weaver-bin/`, gitignored)
and rewrites `internal/semconv/data/registry.json` in place. With no version
change the working tree stays clean.

## Bump the semconv version

1. Edit `SEMCONV_VERSION` in `scripts/resolve-registry.sh` to the new release tag
   (e.g. `v1.42.0`). Bump `WEAVER_VERSION` too only if you are upgrading Weaver.
2. Regenerate and review:
   ```bash
   make semconv-registry
   git diff --stat internal/semconv/data/registry.json
   ```
3. Run the unit tests — the query/registry layer is snapshot-aware:
   ```bash
   make test
   ```
4. Commit `scripts/resolve-registry.sh` **and** the regenerated
   `internal/semconv/data/registry.json` together.

!!! note "Keep the embedded version and any generated dependency refs in lockstep"
    Anything that emits the pinned registry path (for example a generated Weaver
    `registry_manifest.yaml` dependency) must reference the same `SEMCONV_VERSION`.
    Bump them together.

## How CI guards it

The `semconv-registry` job in `.github/workflows/ci.yaml` runs
`make semconv-registry-check`, which resolves a fresh snapshot to a temp file and
fails the build if it differs from the committed `registry.json`. A bump that
forgets to regenerate, or a hand-edited snapshot, fails CI.
