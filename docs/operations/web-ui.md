# Web UI — SemConv Explorer

The proxy serves an embedded, responsive single-page UI — **SemConv Explorer** —
at the root path (`/`) of the **API port** (`8080` by default). It is bundled
into the binary with `go:embed`, so there is nothing extra to deploy and no
runtime build step.

```
http://<host>:8080/
```

## What it does

SemConv Explorer has four scopes, toggled by the scope switch in the header:

- **Community** — the official OpenTelemetry registry embedded in the proxy
  (build-time pinned snapshot). Backed by
  [`GET /api/v1/semconv/community`](../api/community.md).
- **My Telemetry** — the attributes the proxy has actually observed on the wire.
  Backed by [`GET /api/v1/dictionary`](../api/dictionary.md).
- **Compare** — your live telemetry bucketed against the official registry
  (matched / type-mismatch / deprecated / not-in-registry). Backed by
  [`GET /api/v1/semconv/compare`](../api/builder.md#compare).
- **Weaver Asset Builder** — author and export a custom Weaver registry from the
  observed telemetry. See [Weaver Asset Builder](weaver-builder.md).

The **Community** and **My Telemetry** scopes share the same faceted search
experience:

- free-text search across name and brief;
- facets for **signal type** (attribute / metric / span / event / entity),
  **stability** (stable / development / release candidate), and **namespace**,
  with live counts;
- a detail view per item (brief, note, stability, deprecation, enum members,
  examples, requirement level) with shareable deep links carried in the URL hash.

The layout is responsive, so the same UI works on a laptop or a phone. The
selected scope (and selection) is carried in the URL hash, so any view is
shareable as a deep link.

## Accessing it

=== "Local / Docker"

    The UI is on the same port as the REST API. With the default config:

    ```bash
    curl -fsS http://localhost:8080/healthz   # liveness
    open http://localhost:8080/               # SemConv Explorer
    ```

=== "Kubernetes (port-forward)"

    ```bash
    kubectl port-forward svc/<release>-semconv-proxy 8080:8080
    # then open http://localhost:8080/
    ```

=== "Kubernetes (Ingress)"

    Expose the API port through an Ingress or Gateway and the UI is reachable at
    the ingress host root. See
    [Kubernetes / Helm → Exposing the UI](kubernetes.md#exposing-the-ui).

## Notes

- The UI and the REST API share the API port; OTLP ingest stays on its own
  ports (`4317` gRPC, `4318` HTTP) and is never served the UI.
- Unknown non-`/api/` paths fall back to the SPA shell so client-side deep links
  resolve. Unknown `/api/` paths return a JSON `404`, never the HTML shell.
