#!/usr/bin/env bash
#
# Regenerate the embedded OpenTelemetry semantic-convention snapshot
# (internal/semconv/data/registry.json) from a PINNED semconv release using a
# PINNED Weaver binary. Weaver runs at build time only — the proxy binary has no
# runtime dependency on Weaver or network access.
#
# Usage:
#   scripts/resolve-registry.sh            Regenerate registry.json in place.
#   scripts/resolve-registry.sh --check    Resolve to a temp file and fail if it
#                                           differs from the committed snapshot
#                                           (drift check for CI).
#
# To bump the semconv version, edit SEMCONV_VERSION below (and WEAVER_VERSION when
# upgrading Weaver), run this script, and commit the regenerated registry.json.
# See docs/development/semconv-registry.md.
set -euo pipefail

# --- Pinned versions ---------------------------------------------------------
WEAVER_VERSION="${WEAVER_VERSION:-0.23.0}"
SEMCONV_VERSION="${SEMCONV_VERSION:-v1.41.1}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="${REPO_ROOT}/internal/semconv/data/registry.json"
REGISTRY="https://github.com/open-telemetry/semantic-conventions.git@${SEMCONV_VERSION}[model]"
CACHE_DIR="${REPO_ROOT}/.weaver-bin"

log() { printf '>> %s\n' "$*" >&2; }

# --- Resolve the host triple for the Weaver release asset --------------------
host_triple() {
  local os arch
  case "$(uname -s)" in
    Linux)  os="unknown-linux-gnu" ;;
    Darwin) os="apple-darwin" ;;
    *) log "unsupported OS: $(uname -s)"; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64)  arch="x86_64" ;;
    arm64|aarch64) arch="aarch64" ;;
    *) log "unsupported arch: $(uname -m)"; exit 1 ;;
  esac
  printf '%s-%s' "$arch" "$os"
}

# --- Ensure the pinned Weaver binary is available ----------------------------
ensure_weaver() {
  # Prefer the binary bundled with the spike, if it matches the pinned version.
  local spike="${REPO_ROOT}/.spike-weaver/bin/weaver"
  if [ -x "$spike" ] && "$spike" --version 2>/dev/null | grep -q "weaver ${WEAVER_VERSION}"; then
    printf '%s' "$spike"; return
  fi

  local triple bin
  triple="$(host_triple)"
  bin="${CACHE_DIR}/weaver-${WEAVER_VERSION}-${triple}"
  if [ -x "$bin" ]; then printf '%s' "$bin"; return; fi

  log "downloading Weaver ${WEAVER_VERSION} (${triple})"
  mkdir -p "$CACHE_DIR"
  local url tmp
  url="https://github.com/open-telemetry/weaver/releases/download/v${WEAVER_VERSION}/weaver-${triple}.tar.xz"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  curl -fsSL -o "${tmp}/weaver.tar.xz" "$url"
  tar -xJf "${tmp}/weaver.tar.xz" -C "$tmp"
  install -m 0755 "${tmp}/weaver-${triple}/weaver" "$bin"
  printf '%s' "$bin"
}

resolve() {
  local weaver="$1" dest="$2"
  log "resolving ${REGISTRY}"
  "$weaver" registry resolve -r "$REGISTRY" --format json --quiet -o "$dest"
}

main() {
  local weaver
  weaver="$(ensure_weaver)"
  log "using $("$weaver" --version)"

  if [ "${1:-}" = "--check" ]; then
    local tmp drifted=0
    tmp="$(mktemp)"
    resolve "$weaver" "$tmp"
    diff -q "$tmp" "$OUTPUT" >/dev/null 2>&1 || drifted=1
    rm -f "$tmp"
    if [ "$drifted" -eq 1 ]; then
      log "DRIFT: ${OUTPUT} is out of date for semconv ${SEMCONV_VERSION}."
      log "Run 'make semconv-registry' and commit the result."
      exit 1
    fi
    log "registry.json is up to date with semconv ${SEMCONV_VERSION} ✓"
    return
  fi

  resolve "$weaver" "$OUTPUT"
  log "wrote ${OUTPUT}"
}

main "$@"
