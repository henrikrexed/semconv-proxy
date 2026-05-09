# Use Cases

SemConv Proxy addresses real problems that platform, SRE, and development teams face when working with OpenTelemetry semantic conventions.

## Common Scenarios

| Use Case | Persona | What You Get |
|----------|---------|-------------|
| [Semantic Convention Discovery](semantic-discovery.md) | Application Developer | Live dictionary of emitted conventions → Weaver YAML → CI validation |
| [Migration Tracking](migration-tracking.md) | Engineer / SRE | Before/after attribute snapshots to validate instrumentation migrations |
| [Cardinality Management](observability-pipeline.md) | SRE / On-Call | Per-attribute cardinality tracking with Prometheus alerts and budget enforcement |
| [Platform Team Fleet Monitoring](multi-source-aggregation.md) | Platform Engineer | Unified view of conventions across all teams with automated drift detection |

## Who Is This For?

| Persona | Primary Use Case |
|---------|-----------------|
| Platform Engineer | Discover and standardize conventions across services |
| SRE / On-Call | Identify cardinality explosions before they cause incidents |
| Application Developer | Understand what telemetry their service emits |
| Dev Advocate | Demonstrate OTel best practices with real convention data |
