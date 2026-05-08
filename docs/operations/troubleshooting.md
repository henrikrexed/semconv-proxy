# Troubleshooting

## Common Issues

### Signals Not Forwarded

**Symptom:** Signals received but not reaching backend.

**Checks:**
1. Verify backend endpoint: `--backend-endpoint`
2. Check forwarder logs: `--log-level=debug`
3. Verify network connectivity to backend

### Dictionary Empty

**Symptom:** No attributes discovered.

**Checks:**
1. Verify signals are reaching the receiver (check `/metrics`)
2. Check ring buffer: `semconv_proxy_ring_buffer_dropped_total`
3. Verify worker pool is running: `semconv_proxy_worker_pool_active`

### High Memory Usage

**Symptom:** Proxy consuming too much memory.

**Solutions:**
1. Reduce `--dictionary.global-budget`
2. Decrease `--dictionary.ttl`
3. Reduce `--analysis.ring-buffer-size`

### Pebble Errors

**Symptom:** Storage write failures.

**Checks:**
1. Verify disk space on storage path
2. Check file permissions
3. Disable persistence if not needed: `--storage.enabled=false`

### gRPC Connection Refused

**Symptom:** Clients can't connect on port 4317.

**Checks:**
1. Verify `--grpc-port` is correct
2. Check firewall rules
3. Ensure no other process is using the port
