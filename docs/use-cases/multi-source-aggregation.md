# Multi-Source Aggregation — Platform Team Fleet Monitoring

## What This Use Case Achieves

Multi-Source Aggregation gives platform teams unified visibility into semantic convention usage across every service in the organization. By routing all OTel Collectors through a central SemConv Proxy, the dictionary captures every attribute from every team. You can spot naming drift (e.g., `http.method` vs `http.request.method` vs `httpReqMethod`), export a unified Weaver YAML registry as the organizational standard, and set up automated drift detection alerts when new custom attributes appear.

## The Problem

As a platform team, you manage observability infrastructure for dozens of service teams. Each team instruments their services independently. Over time, semantic conventions drift:

- Team A uses `http.request.method` while Team B uses `http.method`
- One team emits `k8s.namespace.name`, another uses `kubernetes.namespace`
- Custom attributes accumulate without standardization
- New team members don't know which conventions to follow

You need visibility into convention drift across all services.

## Fleet Monitoring and Drift Detection Flow

```mermaid
sequenceDiagram
    participant TeamA as Team A<br/>OTel Collector
    participant TeamB as Team B<br/>OTel Collector
    participant TeamC as Team C<br/>OTel Collector
    participant Proxy as SemConv Proxy<br/>(Central)
    participant BE as Backend
    participant Plat as Platform Engineer
    participant Cron as Drift Check (CronJob)

    TeamA->>Proxy: OTLP signals (conventions A)
    TeamB->>Proxy: OTLP signals (conventions B)
    TeamC->>Proxy: OTLP signals (conventions C)
    par Forwarding
        Proxy->>BE: Forward all signals
    and Aggregation
        Proxy->>Proxy: Merge all attributes<br/>into unified dictionary
    end
    Plat->>Proxy: GET /api/v1/dictionary?q=http.*method
    Proxy-->>Plat: http.method, http.request.method, httpReqMethod
    Note over Plat: Drift detected — 3 naming variants
    Plat->>Proxy: GET /api/v1/export?format=weaver
    Proxy-->>Plat: org-semconv-registry.yaml
    Note over Plat: Share registry as standard
    Cron->>Proxy: GET /api/v1/dictionary (daily)
    Cron->>Cron: Compare count vs previous day
    alt >10% growth
        Cron-->>Plat: Alert: new custom attributes detected
    end
```

## The Solution

Deploy SemConv Proxy as a central aggregation point. All OTel Collectors in the organization route through the proxy. The dictionary captures every attribute across every service.

## Step-by-Step Implementation Guide

### Step 1: Deploy as a Central Aggregation Service

```bash
helm install semconv-proxy ./deployments/helm/semconv-proxy/ \
  --set config.backendEndpoint=otel-collector-gateway.observability:4317 \
  --set config.globalBudget=50000 \
  --set config.perAttrCap=2000 \
  --set persistence.enabled=true \
  --set persistence.size=5Gi \
  --set resources.limits.memory=1Gi
```

Note the higher global budget (50K) and per-attr cap (2K) for a platform-wide deployment.

### Step 2: Route All Team Collectors Through the Proxy

Update each OTel Collector's configuration:

```yaml
# Team A's Collector
exporters:
  otlp:
    endpoint: "semconv-proxy.observability:4317"

# Team B's Collector
exporters:
  otlp:
    endpoint: "semconv-proxy.observability:4317"
```

### Step 3: Identify Semantic Convention Drift

Query the dictionary for naming inconsistencies:

```bash
# Find HTTP-related attributes
curl "http://semconv-proxy:8080/api/v1/dictionary?q=http.*method" | \
  jq '.entries[] | {name, signal_types, cardinality}'
```

You might discover:

```json
[
  {"name": "http.request.method", "signal_types": ["metric", "trace"], "cardinality": 6},
  {"name": "http.method", "signal_types": ["trace"], "cardinality": 6},
  {"name": "httpReqMethod", "signal_types": ["log"], "cardinality": 7}
]
```

Three different conventions for the same concept. Time to standardize.

### Step 4: Audit Kubernetes Naming Inconsistencies

```bash
curl "http://semconv-proxy:8080/api/v1/dictionary?q=k8s.*" | \
  jq '.entries[] | {name, cardinality}'
```

### Step 5: Export the Organization-Wide Convention Registry

```bash
curl "http://semconv-proxy:8080/api/v1/export?format=weaver" -o org-semconv-registry.yaml
```

Share this registry with all teams as the source of truth.

### Step 6: Automate Drift Detection with Scheduled Checks

Create a periodic job that checks for new custom attributes:

```bash
#!/bin/bash
# drift-check.sh — run daily via cron
PREVIOUS=$(cat /data/previous-count.txt 2>/dev/null || echo 0)
CURRENT=$(curl -s http://semconv-proxy:8080/api/v1/dictionary | jq '.total')
echo "$CURRENT" > /data/previous-count.txt

if [ "$CURRENT" -gt "$((PREVIOUS + PREVIOUS / 10))" ]; then
  echo "WARNING: Attribute count grew by >10% ($PREVIOUS → $CURRENT)"
  # Send notification
fi
```

## Convention Comparison Workflow

```mermaid
graph TD
    All["All services →<br/>SemConv Proxy"] --> Dict["Live Dictionary<br/>(all attributes)"]
    Dict --> Check["Check for<br/>near-matches"]
    Check --> Standard["OTel Standard<br/>Conventions"]
    Check --> Custom["Custom /<br/>Non-standard"]
    Standard --> Match["✅ Match"]
    Check --> Near["Near-miss<br/>(typo, old version)"]
    Near --> Flag["Flag for<br/>remediation"]
    Custom --> Review["Review: adopt<br/>or replace"]
```

## Dashboard

Build a Grafana dashboard using the proxy's Prometheus metrics:

| Panel | Metric | Purpose |
|-------|--------|---------|
| Total Attributes | `semconv_proxy_dictionary_entries` | Track convention count over time |
| Attributes Added | `rate(semconv_proxy_dictionary_attributes_added_total[1h])` | Detect new conventions being introduced |
| High-Cardinality Count | `semconv_proxy_cardinality_high_attributes` | Monitor cardinality health |
| Budget Utilization | `semconv_proxy_cardinality_budget_utilization` | Track dictionary capacity |
| Pipeline Drops | `rate(semconv_proxy_pipeline_drops_total[5m])` | Check if analysis is keeping up |

## Platform Team Benefits

- **Unified visibility** — one dictionary for the entire organization
- **Drift detection** — spot naming inconsistencies across teams
- **Standardization evidence** — data-driven conversations about convention adoption
- **Cardinality governance** — prevent costly backend explosions before they happen
- **Onboarding** — new teams see what conventions the organization uses
