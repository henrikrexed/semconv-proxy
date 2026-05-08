# Kubernetes / Helm Deployment

## Quick Install

```bash
helm install semconv-proxy ./deployments/helm/semconv-proxy/ \
  --set config.backendEndpoint=otel-collector.observability:4317
```

## Configuration

### Required Values

| Value | Description |
|-------|-------------|
| `config.backendEndpoint` | OTLP backend endpoint (`host:port`) |

### All Values

```yaml
# values.yaml

replicaCount: 1

image:
  repository: ghcr.io/henrikrexed/semconv-proxy
  pullPolicy: IfNotPresent
  tag: "latest"

serviceAccount:
  create: true
  name: ""
  annotations: {}

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  fsGroup: 65532

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

service:
  type: ClusterIP
  ports:
    http: 4318
    grpc: 4317
    api: 8080

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

config:
  backendEndpoint: ""
  logLevel: info
  dataDir: /data
  shardCount: 64
  globalBudget: 10000
  perAttrCap: 1000
  ringBufferSize: 10000

persistence:
  enabled: true
  size: 1Gi
  storageClassName: ""

ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts: []
  tls: []
```

## Persistence

Enable Pebble persistence with a PersistentVolumeClaim:

```yaml
persistence:
  enabled: true
  size: 5Gi
  storageClassName: standard
```

The PVC mounts at `/data` inside the container. Dictionary state survives pod restarts and reschedules.

## Health Probes

The Helm chart configures Kubernetes probes automatically:

```yaml
# Automatically configured in the deployment template
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

## Resource Tuning

### Small Deployment (<1K attributes)

```yaml
resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 64Mi
config:
  globalBudget: 5000
  perAttrCap: 500
```

### Medium Deployment (5-10K attributes)

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
config:
  globalBudget: 10000
  perAttrCap: 1000
```

### Large Deployment (>10K attributes)

```yaml
resources:
  limits:
    cpu: "1"
    memory: 1Gi
  requests:
    cpu: 250m
    memory: 256Mi
config:
  globalBudget: 50000
  perAttrCap: 2000
  shardCount: 128
  ringBufferSize: 50000
persistence:
  size: 10Gi
```

## Ingress

Expose the API externally (not recommended for production — keep the API on internal network):

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: semconv-proxy.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: semconv-proxy-tls
      hosts:
        - semconv-proxy.example.com
```

## Upgrading

```bash
helm upgrade semconv-proxy ./deployments/helm/semconv-proxy/ \
  --set config.backendEndpoint=otel-collector.observability:4317 \
  --set config.globalBudget=50000
```

The proxy performs graceful shutdown: it drains the ring buffer and persists the dictionary before terminating.

## Uninstalling

```bash
helm uninstall semconv-proxy
```

If persistence was enabled, the PVC remains unless manually deleted:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=semconv-proxy
```
