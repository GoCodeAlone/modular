# Health Aggregation

## Overview

The Health Aggregation feature provides a unified health checking system for Modular applications. It collects health status from multiple modules, aggregates the results, and provides distinct readiness and liveness endpoints for orchestration platforms like Kubernetes.

## Quick Start

### 1. Enable Health Aggregation

```go
package main

import (
    "github.com/GoCodeAlone/modular"
)

func main() {
    app := modular.NewApplication(
        modular.WithHealthAggregator(), // Enable with defaults
    )
    
    // Register modules with health providers...
    app.Run()
}
```

### 2. Implement Health Provider

```go
type DatabaseModule struct {
    db *sql.DB
}

func (m *DatabaseModule) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    reports := []modular.HealthReport{}
    
    // Check database connection
    if err := m.db.PingContext(ctx); err != nil {
        reports = append(reports, modular.HealthReport{
            Module:    "database",
            Component: "connection",
            Status:    modular.HealthStatusUnhealthy,
            Message:   fmt.Sprintf("Database ping failed: %v", err),
            CheckedAt: time.Now(),
        })
    } else {
        reports = append(reports, modular.HealthReport{
            Module:    "database",
            Component: "connection",
            Status:    modular.HealthStatusHealthy,
            Message:   "Database connection healthy",
            CheckedAt: time.Now(),
            Details: map[string]any{
                "connections_open": m.db.Stats().OpenConnections,
                "connections_idle": m.db.Stats().Idle,
            },
        })
    }
    
    return reports, nil
}
```

### 3. Register Health Provider

```go
func (m *DatabaseModule) Init(app modular.Application) error {
    // Register as a required health provider
    app.RegisterHealthProvider("database", m, false)
    
    // Or register as optional (won't affect readiness)
    // app.RegisterHealthProvider("metrics", m, true)
    
    return nil
}
```

## Configuration

### Basic Configuration

```go
app := modular.NewApplication(
    modular.WithHealthAggregator(
        modular.HealthAggregatorConfig{
            // Cache health results for 250ms
            CacheDuration: 250 * time.Millisecond,
            
            // Timeout individual health checks after 200ms
            Timeout: 200 * time.Millisecond,
            
            // Enable result caching
            EnableCache: true,
        },
    ),
)
```

### Advanced Configuration

```go
app := modular.NewApplication(
    modular.WithHealthAggregator(
        modular.HealthAggregatorConfig{
            CacheDuration: 500 * time.Millisecond,
            Timeout:       1 * time.Second,
            EnableCache:   true,
            
            // Custom aggregation rules
            AggregationStrategy: modular.HealthAggregationStrict,
            
            // Include detailed component reports
            IncludeDetails: true,
        },
    ),
)
```

## Health Status Types

### Status Levels

| Status | Description | HTTP Code |
|--------|-------------|-----------|
| `Healthy` | Component operating normally | 200 |
| `Degraded` | Operational but impaired | 200 |
| `Unhealthy` | Component not functioning | 503 |
| `Unknown` | Status cannot be determined | 503 |

### Readiness vs Liveness

**Readiness**: Can the service accept traffic?
- Only considers required (non-optional) components
- Used by load balancers to route traffic
- Endpoint: `/ready`

**Liveness**: Is the service running?
- Considers all components (required and optional)
- Used by orchestrators for restart decisions
- Endpoint: `/health`

## HTTP Endpoints

### Health Check Endpoint

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "readiness": "healthy",
  "health": "degraded",
  "timestamp": "2024-01-15T10:30:00Z",
  "reports": [
    {
      "module": "database",
      "component": "connection",
      "status": "healthy",
      "message": "Database connection healthy",
      "checkedAt": "2024-01-15T10:30:00Z",
      "details": {
        "connections_open": 5,
        "connections_idle": 2
      }
    },
    {
      "module": "cache",
      "status": "degraded",
      "message": "High memory usage",
      "optional": true,
      "checkedAt": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Readiness Endpoint

```bash
GET /ready
```

Response (only required components):
```json
{
  "ready": true,
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Liveness Endpoint

```bash
GET /alive
```

Response:
```json
{
  "alive": true,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Implementation Examples

### Database Health Check

```go
type DatabaseHealth struct {
    db     *sql.DB
    config *DatabaseConfig
}

func (h *DatabaseHealth) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    reports := []modular.HealthReport{}
    
    // Basic connectivity check
    checkStart := time.Now()
    err := h.db.PingContext(ctx)
    checkDuration := time.Since(checkStart)
    
    if err != nil {
        return []modular.HealthReport{{
            Module:    "database",
            Component: "connectivity",
            Status:    modular.HealthStatusUnhealthy,
            Message:   fmt.Sprintf("Ping failed: %v", err),
            CheckedAt: time.Now(),
            Details: map[string]any{
                "error":    err.Error(),
                "duration": checkDuration.String(),
            },
        }}, nil
    }
    
    // Check connection pool health
    stats := h.db.Stats()
    poolHealth := modular.HealthStatusHealthy
    poolMessage := "Connection pool healthy"
    
    utilizationPct := float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
    if utilizationPct > 90 {
        poolHealth = modular.HealthStatusDegraded
        poolMessage = fmt.Sprintf("High connection pool utilization: %.1f%%", utilizationPct)
    }
    
    reports = append(reports, 
        modular.HealthReport{
            Module:    "database",
            Component: "connectivity",
            Status:    modular.HealthStatusHealthy,
            Message:   "Database reachable",
            CheckedAt: time.Now(),
            Details: map[string]any{
                "latency": checkDuration.String(),
            },
        },
        modular.HealthReport{
            Module:    "database",
            Component: "connection_pool",
            Status:    poolHealth,
            Message:   poolMessage,
            CheckedAt: time.Now(),
            Details: map[string]any{
                "max_connections":  stats.MaxOpenConnections,
                "open_connections": stats.OpenConnections,
                "in_use":          stats.InUse,
                "idle":            stats.Idle,
                "utilization_pct": utilizationPct,
            },
        },
    )
    
    return reports, nil
}
```

### Cache Health Check

```go
type CacheHealth struct {
    client *redis.Client
    config *CacheConfig
}

func (h *CacheHealth) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    // Check Redis connectivity
    pong, err := h.client.Ping(ctx).Result()
    if err != nil {
        return []modular.HealthReport{{
            Module:    "cache",
            Status:    modular.HealthStatusUnhealthy,
            Message:   fmt.Sprintf("Redis ping failed: %v", err),
            CheckedAt: time.Now(),
            Optional:  true, // Cache is optional for basic operation
        }}, nil
    }
    
    // Check memory usage
    info, err := h.client.Info(ctx, "memory").Result()
    if err != nil {
        return []modular.HealthReport{{
            Module:    "cache",
            Status:    modular.HealthStatusDegraded,
            Message:   "Could not retrieve memory stats",
            CheckedAt: time.Now(),
            Optional:  true,
        }}, nil
    }
    
    // Parse memory usage and determine health
    memoryUsed := parseMemoryUsed(info)
    memoryMax := h.config.MaxMemory
    
    status := modular.HealthStatusHealthy
    message := "Cache operating normally"
    
    if memoryMax > 0 {
        usagePct := float64(memoryUsed) / float64(memoryMax) * 100
        if usagePct > 90 {
            status = modular.HealthStatusDegraded
            message = fmt.Sprintf("High memory usage: %.1f%%", usagePct)
        } else if usagePct > 95 {
            status = modular.HealthStatusUnhealthy
            message = fmt.Sprintf("Critical memory usage: %.1f%%", usagePct)
        }
    }
    
    return []modular.HealthReport{{
        Module:    "cache",
        Status:    status,
        Message:   message,
        CheckedAt: time.Now(),
        Optional:  true,
        Details: map[string]any{
            "ping_response": pong,
            "memory_used":   memoryUsed,
            "memory_max":    memoryMax,
        },
    }}, nil
}
```

### HTTP Service Health Check

```go
type HTTPServiceHealth struct {
    client  *http.Client
    config  *ServiceConfig
}

func (h *HTTPServiceHealth) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    reports := []modular.HealthReport{}
    
    for _, endpoint := range h.config.HealthEndpoints {
        report := h.checkEndpoint(ctx, endpoint)
        reports = append(reports, report)
    }
    
    return reports, nil
}

func (h *HTTPServiceHealth) checkEndpoint(ctx context.Context, endpoint string) modular.HealthReport {
    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return modular.HealthReport{
            Module:    "http_service",
            Component: endpoint,
            Status:    modular.HealthStatusUnhealthy,
            Message:   fmt.Sprintf("Failed to create request: %v", err),
            CheckedAt: time.Now(),
        }
    }
    
    start := time.Now()
    resp, err := h.client.Do(req)
    latency := time.Since(start)
    
    if err != nil {
        return modular.HealthReport{
            Module:    "http_service",
            Component: endpoint,
            Status:    modular.HealthStatusUnhealthy,
            Message:   fmt.Sprintf("Request failed: %v", err),
            CheckedAt: time.Now(),
            Details: map[string]any{
                "error":   err.Error(),
                "latency": latency.String(),
            },
        }
    }
    defer resp.Body.Close()
    
    status := modular.HealthStatusHealthy
    message := fmt.Sprintf("Endpoint responding (HTTP %d)", resp.StatusCode)
    
    if resp.StatusCode >= 500 {
        status = modular.HealthStatusUnhealthy
        message = fmt.Sprintf("Server error: HTTP %d", resp.StatusCode)
    } else if resp.StatusCode >= 400 {
        status = modular.HealthStatusDegraded
        message = fmt.Sprintf("Client error: HTTP %d", resp.StatusCode)
    } else if latency > h.config.LatencyThreshold {
        status = modular.HealthStatusDegraded
        message = fmt.Sprintf("High latency: %v", latency)
    }
    
    return modular.HealthReport{
        Module:    "http_service",
        Component: endpoint,
        Status:    status,
        Message:   message,
        CheckedAt: time.Now(),
        Details: map[string]any{
            "status_code": resp.StatusCode,
            "latency":     latency.String(),
        },
    }
}
```

## Event Monitoring

Subscribe to health events for monitoring and alerting:

```go
app.RegisterObserver(func(ctx context.Context, event modular.CloudEvent) error {
    switch event.Type() {
    case "health.evaluated":
        data := event.Data().(HealthEvaluatedEvent)
        if data.StatusChanged {
            log.Warn("Health status changed",
                "from", data.PreviousStatus,
                "to", data.Snapshot.Health)
        }
        
    case "health.degraded":
        log.Warn("Service health degraded", "details", event.Data())
        // Send alert...
        
    case "health.recovered":
        log.Info("Service health recovered")
        // Clear alert...
    }
    return nil
})
```

## Best Practices

### 1. Keep Health Checks Fast

Health checks should complete quickly to avoid timeouts:

```go
func (m *Module) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    // Use context with timeout
    checkCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
    defer cancel()
    
    // Perform quick check
    if err := m.quickPing(checkCtx); err != nil {
        return []modular.HealthReport{{
            Module:  m.Name(),
            Status:  modular.HealthStatusUnhealthy,
            Message: "Quick check failed",
        }}, nil
    }
    
    // Don't do expensive operations in health checks
    // ❌ Avoid: Full table scans, complex queries, large data transfers
    // ✅ Prefer: Simple pings, connection checks, quick stats queries
    
    return []modular.HealthReport{{
        Module:  m.Name(),
        Status:  modular.HealthStatusHealthy,
        Message: "Operating normally",
    }}, nil
}
```

### 2. Use Optional for Non-Critical Components

Mark components as optional if they don't affect core functionality:

```go
// Register optional components that won't affect readiness
app.RegisterHealthProvider("metrics", metricsProvider, true)    // Optional
app.RegisterHealthProvider("cache", cacheProvider, true)        // Optional
app.RegisterHealthProvider("database", dbProvider, false)       // Required
```

### 3. Include Meaningful Details

Provide useful debugging information in health reports:

```go
return []modular.HealthReport{{
    Module:    "api",
    Component: "rate_limiter",
    Status:    modular.HealthStatusDegraded,
    Message:   "Rate limit approaching threshold",
    CheckedAt: time.Now(),
    Details: map[string]any{
        "current_rate":    850,
        "limit":          1000,
        "utilization_pct": 85,
        "window":         "1m",
        "reset_at":       time.Now().Add(15 * time.Second),
    },
}}
```

### 4. Handle Timeouts Gracefully

Always respect context cancellation:

```go
func (m *Module) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    resultCh := make(chan []modular.HealthReport, 1)
    errCh := make(chan error, 1)
    
    go func() {
        reports, err := m.performHealthCheck()
        if err != nil {
            errCh <- err
        } else {
            resultCh <- reports
        }
    }()
    
    select {
    case <-ctx.Done():
        return []modular.HealthReport{{
            Module:  m.Name(),
            Status:  modular.HealthStatusUnknown,
            Message: "Health check timed out",
        }}, nil
    case err := <-errCh:
        return nil, err
    case reports := <-resultCh:
        return reports, nil
    }
}
```

### 5. Cache Expensive Checks

For expensive health checks, implement caching:

```go
type CachedHealthProvider struct {
    mu           sync.RWMutex
    lastCheck    time.Time
    lastReports  []modular.HealthReport
    cacheTTL     time.Duration
    actualCheck  func(context.Context) ([]modular.HealthReport, error)
}

func (p *CachedHealthProvider) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
    p.mu.RLock()
    if time.Since(p.lastCheck) < p.cacheTTL {
        reports := p.lastReports
        p.mu.RUnlock()
        return reports, nil
    }
    p.mu.RUnlock()
    
    // Perform actual check
    reports, err := p.actualCheck(ctx)
    if err != nil {
        return nil, err
    }
    
    // Update cache
    p.mu.Lock()
    p.lastReports = reports
    p.lastCheck = time.Now()
    p.mu.Unlock()
    
    return reports, nil
}
```

## Kubernetes Integration

### Deployment Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: modular-app
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        
        # Liveness probe - restart if unhealthy
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        
        # Readiness probe - stop routing traffic if not ready
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
```

### Startup Probe (Kubernetes 1.16+)

For slow-starting applications:

```yaml
startupProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 0
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 30  # 5 minutes to start
```

## Monitoring and Alerting

### Prometheus Metrics

The health system exposes Prometheus metrics:

```prometheus
# Health check duration
health_check_duration_seconds{module="database",component="connection"} 0.002

# Health status (0=unknown, 1=healthy, 2=degraded, 3=unhealthy)
health_status{module="database",component="connection"} 1

# Aggregated health
health_aggregated_status{type="readiness"} 1
health_aggregated_status{type="liveness"} 2
```

### Example Alerts

```yaml
groups:
- name: health
  rules:
  - alert: ServiceUnhealthy
    expr: health_aggregated_status{type="readiness"} > 1
    for: 5m
    annotations:
      summary: "Service {{ $labels.service }} is unhealthy"
      
  - alert: ComponentDegraded
    expr: health_status == 2
    for: 15m
    annotations:
      summary: "Component {{ $labels.module }}/{{ $labels.component }} degraded"
```

## Troubleshooting

### Common Issues

#### Health Checks Timing Out

**Symptoms:**
- Health endpoints return 503
- Logs show timeout errors

**Solutions:**
1. Increase timeout configuration
2. Optimize health check queries
3. Use caching for expensive checks
4. Check network connectivity

#### Inconsistent Health Status

**Symptoms:**
- Health status flapping between healthy/unhealthy
- Kubernetes repeatedly restarting pods

**Solutions:**
1. Increase failure thresholds in probes
2. Add hysteresis to health checks
3. Review health check logic for race conditions
4. Check for transient network issues

#### High CPU from Health Checks

**Symptoms:**
- CPU spikes during health checks
- Slow response times

**Solutions:**
1. Enable caching in health aggregator
2. Reduce health check frequency
3. Optimize individual health checks
4. Use connection pooling

### Debug Mode

Enable detailed health check logging:

```go
app := modular.NewApplication(
    modular.WithHealthAggregator(
        modular.HealthAggregatorConfig{
            DebugMode: true,
            LogLevel:  "debug",
        },
    ),
)
```

## Performance Considerations

1. **Caching**: Default 250ms cache prevents redundant checks
2. **Concurrency**: Health checks run in parallel with individual timeouts
3. **Circuit Breaking**: Failed providers are temporarily skipped
4. **Resource Impact**: Keep health checks lightweight

## Security

1. **Authentication**: Protect health endpoints in production
2. **Information Disclosure**: Limit details in public endpoints
3. **Rate Limiting**: Prevent health check abuse
4. **Internal vs External**: Separate internal detailed checks from public status

Example secure configuration:

```go
// Public endpoint - minimal information
router.GET("/health", publicHealthHandler)

// Internal endpoint - full details  
router.GET("/internal/health", 
    authMiddleware,
    detailedHealthHandler)
```