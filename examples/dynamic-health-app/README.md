# Dynamic Health Application Example

This example demonstrates the integrated use of Dynamic Configuration Reload and Health Aggregation features in a Modular application.

## Features Demonstrated

- **Dynamic Configuration Reload**: Update connection pools, timeouts, and feature flags without restart
- **Health Aggregation**: Unified health checking across database, cache, and application components
- **Kubernetes-Ready**: Separate readiness and liveness endpoints
- **Circuit Breaker**: Automatic backoff for failed configuration reloads
- **Event Monitoring**: CloudEvents for configuration and health status changes

## Architecture

```
┌─────────────────────────────────────────────────┐
│                 HTTP Server                      │
│  /health  /ready  /alive  /reload  /config      │
└─────────────────┬───────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
┌───────▼────────┐ ┌────────▼────────┐
│ Database Module│ │  Cache Module   │
│                │ │                 │
│ - Health Check │ │ - Health Check  │
│ - Dynamic Pool │ │ - Dynamic TTL   │
│ - Connections  │ │ - Size Limits   │
└────────────────┘ └─────────────────┘
```

## Running the Example

### Prerequisites

- Go 1.21 or higher
- PostgreSQL (optional, for full functionality)
- Docker (optional, for containerized PostgreSQL)

### Quick Start

1. **Start PostgreSQL** (optional):
```bash
docker run -d \
  --name modular-postgres \
  -e POSTGRES_DB=myapp \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:15
```

2. **Run the application**:
```bash
go run main.go
```

3. **Check health status**:
```bash
# Full health report
curl http://localhost:8080/health | jq

# Readiness check (for load balancers)
curl http://localhost:8080/ready

# Liveness check (for orchestrators)
curl http://localhost:8080/alive
```

## Dynamic Configuration Updates

### Updating Configuration

1. **Edit config.yaml** while the application is running:
```yaml
database:
  max_connections: 50    # Increased from 25
  max_idle_conns: 10     # Increased from 5
```

2. **Trigger reload**:
```bash
curl -X POST http://localhost:8080/reload
```

3. **Verify changes**:
```bash
curl http://localhost:8080/health | jq '.reports[] | select(.module=="database")'
```

### Environment Variable Updates

```bash
# Update environment variables
export DB_MAX_CONNS=100
export CACHE_TTL=10m
export LOG_LEVEL=debug

# Trigger reload
curl -X POST http://localhost:8080/reload
```

## Health Check Details

### Database Health

The database module reports:
- **Connectivity**: Basic ping test
- **Connection Pool**: Utilization metrics and health status

```json
{
  "module": "database",
  "component": "connection_pool",
  "status": "healthy",
  "message": "Connection pool healthy",
  "details": {
    "max_connections": 25,
    "connections_open": 5,
    "connections_idle": 2,
    "connections_inuse": 3,
    "utilization_pct": 12.0
  }
}
```

### Cache Health

The cache module reports:
- **Capacity**: Current utilization
- **Status**: Optional component (doesn't affect readiness)

```json
{
  "module": "cache",
  "status": "healthy",
  "message": "Cache operational (42 entries)",
  "optional": true,
  "details": {
    "entries": 42,
    "max_entries": 1000,
    "utilization_pct": 4.2,
    "ttl": "5m0s"
  }
}
```

## Testing Scenarios

### 1. Simulate Database Issues

Stop PostgreSQL to see health degradation:
```bash
docker stop modular-postgres

# Check health
curl http://localhost:8080/health
# Returns 503 with unhealthy status

# Check readiness
curl http://localhost:8080/ready
# Returns 503 - not ready for traffic
```

### 2. Test Dynamic Pool Adjustment

Monitor connection pool during load:
```bash
# Generate load (in another terminal)
for i in {1..100}; do
  curl http://localhost:8080/health &
done

# Increase pool size dynamically
# Edit config.yaml: max_connections: 50
curl -X POST http://localhost:8080/reload

# Check updated pool metrics
curl http://localhost:8080/health | jq '.reports[] | select(.component=="connection_pool")'
```

### 3. Test Circuit Breaker

Force reload failures to trigger backoff:
```bash
# Corrupt config file temporarily
echo "invalid: [yaml" >> config.yaml

# Try multiple reloads
for i in {1..5}; do
  curl -X POST http://localhost:8080/reload
  sleep 1
done
# Later attempts will show "backing off" error

# Fix config and wait for backoff to expire
git checkout config.yaml
sleep 30
curl -X POST http://localhost:8080/reload
```

### 4. Cache Capacity Management

Test dynamic cache size adjustment:
```bash
# Fill cache near capacity
# (Application logic would need to populate cache)

# Check cache health
curl http://localhost:8080/health | jq '.reports[] | select(.module=="cache")'

# If utilization > 80%, increase capacity
# Edit config.yaml: max_entries: 2000
curl -X POST http://localhost:8080/reload
```

## Kubernetes Deployment

### Deployment Manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dynamic-health-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: dynamic-health
  template:
    metadata:
      labels:
        app: dynamic-health
    spec:
      containers:
      - name: app
        image: dynamic-health-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: postgres-service
        - name: CACHE_ENABLED
          value: "true"
        
        # Health checks
        livenessProbe:
          httpGet:
            path: /alive
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          
        # Resource limits
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
```

### ConfigMap for Dynamic Updates

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  config.yaml: |
    database:
      max_connections: 25
      max_idle_conns: 5
    cache:
      enabled: true
      ttl: 5m
      max_entries: 1000
```

Update configuration without pod restart:
```bash
# Update ConfigMap
kubectl edit configmap app-config

# Trigger reload via port-forward
kubectl port-forward deployment/dynamic-health-app 8080:8080
curl -X POST http://localhost:8080/reload
```

## Monitoring

### Prometheus Metrics

The application exposes metrics that can be scraped:

```prometheus
# Health status by module
health_status{module="database",component="connection_pool"} 1
health_status{module="cache"} 1

# Reload metrics
config_reload_total 15
config_reload_failures_total 2
config_reload_duration_seconds 0.125

# Connection pool metrics
db_connections_open 5
db_connections_idle 2
db_connections_inuse 3
```

### Grafana Dashboard

Key panels for monitoring:
1. **Health Status Overview**: Aggregated readiness/liveness
2. **Module Health Matrix**: Individual component status
3. **Configuration Reload History**: Success/failure timeline
4. **Connection Pool Utilization**: Real-time pool metrics
5. **Cache Hit Rate**: Cache effectiveness

## Troubleshooting

### Common Issues

1. **"Service Unavailable" on /health**
   - Check database connectivity
   - Verify required modules are healthy
   - Check logs for specific errors

2. **"Backing off" on reload**
   - Previous reload failed
   - Check configuration validity
   - Wait for backoff period or restart

3. **High connection pool utilization**
   - Increase max_connections dynamically
   - Check for connection leaks
   - Review query performance

4. **Cache degradation**
   - Monitor utilization percentage
   - Increase max_entries if needed
   - Adjust TTL for better hit rate

## Code Structure

```
dynamic-health-app/
├── main.go           # Main application
├── config.yaml       # Configuration file
├── README.md         # This file
└── docker-compose.yml # Optional: Full stack setup
```

## Key Learnings

1. **Separation of Concerns**: Health providers are independent of reload logic
2. **Optional vs Required**: Cache is optional, database is required for readiness
3. **Graceful Degradation**: Service continues with degraded components
4. **Zero-Downtime Updates**: Configuration changes without restart
5. **Circuit Breaker Pattern**: Prevents reload storms after failures

## Next Steps

- Add more modules (message queue, external APIs)
- Implement custom health check logic
- Add Prometheus metrics endpoint
- Create custom CloudEvent handlers
- Implement configuration validation webhooks