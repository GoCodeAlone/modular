# Dynamic Configuration Reload

## Overview

The Dynamic Configuration Reload feature allows your Modular application to update configuration values at runtime without requiring a full restart. This enables zero-downtime configuration changes for supported fields.

## Quick Start

### 1. Enable Dynamic Reload

```go
package main

import (
    "github.com/GoCodeAlone/modular"
)

func main() {
    app := modular.NewApplication(
        modular.WithDynamicReload(),  // Enable dynamic reload with defaults
    )
    
    // Register your modules...
    app.Run()
}
```

### 2. Mark Fields as Dynamic

Use the `dynamic:"true"` tag on configuration fields that should be reloadable:

```go
type DatabaseConfig struct {
    Host     string        `yaml:"host" env:"DB_HOST"`
    Port     int          `yaml:"port" env:"DB_PORT"`
    
    // These fields can be reloaded without restart
    MaxConns int          `yaml:"max_conns" env:"DB_MAX_CONNS" dynamic:"true"`
    Timeout  time.Duration `yaml:"timeout" env:"DB_TIMEOUT" dynamic:"true"`
    LogLevel string       `yaml:"log_level" env:"LOG_LEVEL" dynamic:"true"`
}
```

### 3. Implement Reloadable Interface

For modules that need to respond to configuration changes:

```go
type DatabaseModule struct {
    config *DatabaseConfig
    pool   *sql.DB
}

func (m *DatabaseModule) CanReload() bool {
    return true
}

func (m *DatabaseModule) ReloadTimeout() time.Duration {
    return 5 * time.Second
}

func (m *DatabaseModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    // Handle specific configuration changes
    for _, change := range changes {
        switch change.FieldPath {
        case "database.max_conns":
            if newMax, ok := change.NewValue.(int); ok {
                m.pool.SetMaxOpenConns(newMax)
            }
        case "database.timeout":
            // Update timeout settings
        case "database.log_level":
            // Update logging configuration
        }
    }
    return nil
}
```

## Configuration Options

### Basic Configuration

```go
app := modular.NewApplication(
    modular.WithDynamicReload(
        modular.DynamicReloadConfig{
            // Check for config changes every 30 seconds
            CheckInterval: 30 * time.Second,
            
            // Fail reload if any module doesn't respond within 10 seconds
            ReloadTimeout: 10 * time.Second,
            
            // Enable automatic rollback on failure
            EnableRollback: true,
        },
    ),
)
```

### Advanced Configuration with Circuit Breaker

```go
app := modular.NewApplication(
    modular.WithDynamicReload(
        modular.DynamicReloadConfig{
            CheckInterval:  30 * time.Second,
            ReloadTimeout:  10 * time.Second,
            EnableRollback: true,
            
            // Circuit breaker settings
            BackoffBase: 1 * time.Second,  // Initial backoff duration
            BackoffCap:  30 * time.Second, // Maximum backoff duration
        },
    ),
)
```

## Triggering Reloads

### Manual Reload

```go
// Reload all dynamic configuration
err := app.RequestReload(ctx)

// Reload specific configuration sections
err := app.RequestReload(ctx, "database", "cache")
```

### File-Based Auto-Reload

When using file-based configuration (YAML, JSON, TOML), the system automatically detects changes:

```yaml
# config.yaml
database:
  host: localhost
  port: 5432
  max_conns: 100  # dynamic: true - can be changed without restart
  timeout: 5s     # dynamic: true
```

### Environment Variable Reload

For environment-based configuration, trigger reload after updating variables:

```bash
# Update environment variable
export DB_MAX_CONNS=200

# Signal application to reload (via API or signal)
curl -X POST http://localhost:8080/admin/reload
```

## Event Monitoring

The reload system emits CloudEvents for monitoring:

```go
// Subscribe to reload events
app.RegisterObserver(func(ctx context.Context, event modular.CloudEvent) error {
    switch event.Type() {
    case "reload.started":
        log.Info("Configuration reload started")
    case "reload.completed":
        log.Info("Configuration reload completed successfully")
    case "reload.failed":
        log.Error("Configuration reload failed", "error", event.Data())
    }
    return nil
})
```

## Best Practices

### 1. Identify Dynamic Fields

Only mark fields as dynamic if they can be safely changed at runtime:

✅ **Good Candidates:**
- Connection pool sizes
- Timeouts and intervals  
- Log levels
- Feature flags
- Rate limits

❌ **Not Suitable:**
- Database connection strings
- Server ports
- TLS certificates (use separate cert reload)
- Fundamental architecture settings

### 2. Implement Atomic Updates

Ensure configuration updates are atomic:

```go
func (m *Module) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    // Create new configuration
    newConfig := m.config.Clone()
    
    // Apply all changes
    for _, change := range changes {
        if err := applyChange(newConfig, change); err != nil {
            return err // Rollback on any error
        }
    }
    
    // Validate new configuration
    if err := newConfig.Validate(); err != nil {
        return err
    }
    
    // Atomic swap
    m.mu.Lock()
    m.config = newConfig
    m.mu.Unlock()
    
    return nil
}
```

### 3. Handle Reload Failures

Implement proper rollback logic:

```go
func (m *Module) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    // Save current state for rollback
    oldConfig := m.config.Clone()
    
    // Attempt reload
    if err := m.applyChanges(changes); err != nil {
        // Rollback on failure
        m.config = oldConfig
        m.reinitialize()
        return fmt.Errorf("reload failed, rolled back: %w", err)
    }
    
    return nil
}
```

### 4. Monitor Reload Health

Use the circuit breaker to prevent reload storms:

```go
// The system automatically backs off after failures
// Monitor these metrics:
- reload_attempts_total
- reload_failures_total
- reload_backoff_seconds
- reload_duration_seconds
```

## Troubleshooting

### Common Issues

#### 1. Reload Not Detecting Changes

**Symptom:** Configuration file changes aren't being picked up

**Solutions:**
- Verify file watcher is enabled in configuration
- Check file permissions
- Ensure `dynamic:"true"` tags are present
- Verify module implements `Reloadable` interface

#### 2. Reload Failing with Timeout

**Symptom:** Reload operations timing out

**Solutions:**
- Increase `ReloadTimeout` in configuration
- Check module's `ReloadTimeout()` method
- Verify modules aren't blocking in `Reload()`
- Check for deadlocks in configuration updates

#### 3. Circuit Breaker Activated

**Symptom:** Getting "backing off" errors

**Solutions:**
- Check logs for root cause of failures
- Fix underlying configuration issues
- Wait for backoff period to expire
- Manually reset circuit breaker if needed

### Debug Logging

Enable debug logging for detailed reload information:

```go
app := modular.NewApplication(
    modular.WithDynamicReload(),
    modular.WithLogger(logger.WithLevel("debug")),
)
```

## Examples

### Database Connection Pool

```go
type DatabaseModule struct {
    config *DBConfig
    pool   *sql.DB
    mu     sync.RWMutex
}

func (m *DatabaseModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    for _, change := range changes {
        switch change.FieldPath {
        case "database.max_open_conns":
            if v, ok := change.NewValue.(int); ok {
                m.pool.SetMaxOpenConns(v)
                log.Info("Updated max connections", "value", v)
            }
        case "database.max_idle_conns":
            if v, ok := change.NewValue.(int); ok {
                m.pool.SetMaxIdleConns(v)
                log.Info("Updated max idle connections", "value", v)
            }
        case "database.conn_max_lifetime":
            if v, ok := change.NewValue.(time.Duration); ok {
                m.pool.SetConnMaxLifetime(v)
                log.Info("Updated connection lifetime", "value", v)
            }
        }
    }
    return nil
}
```

### Feature Flags

```go
type FeatureFlags struct {
    EnableNewUI      bool `json:"enable_new_ui" dynamic:"true"`
    EnableBetaAPI    bool `json:"enable_beta_api" dynamic:"true"`
    MaintenanceMode  bool `json:"maintenance_mode" dynamic:"true"`
}

func (m *AppModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for _, change := range changes {
        switch change.FieldPath {
        case "features.maintenance_mode":
            if enabled, ok := change.NewValue.(bool); ok && enabled {
                m.enterMaintenanceMode()
            } else {
                m.exitMaintenanceMode()
            }
        }
    }
    return nil
}
```

### Rate Limiting

```go
type RateLimitConfig struct {
    RequestsPerSecond int           `yaml:"requests_per_second" dynamic:"true"`
    BurstSize        int           `yaml:"burst_size" dynamic:"true"`
    WindowDuration   time.Duration `yaml:"window_duration" dynamic:"true"`
}

func (m *RateLimiter) Reload(ctx context.Context, changes []modular.ConfigChange) error {
    // Create new rate limiter with updated settings
    newLimiter := rate.NewLimiter(
        rate.Limit(m.config.RequestsPerSecond),
        m.config.BurstSize,
    )
    
    // Atomic swap
    m.mu.Lock()
    m.limiter = newLimiter
    m.mu.Unlock()
    
    return nil
}
```

## API Reference

### Interfaces

```go
// Reloadable interface for modules that support configuration reload
type Reloadable interface {
    // CanReload indicates if the module supports reload
    CanReload() bool
    
    // ReloadTimeout returns the maximum time to wait for reload
    ReloadTimeout() time.Duration
    
    // Reload applies configuration changes
    Reload(ctx context.Context, changes []ConfigChange) error
}

// ConfigChange represents a single configuration field change
type ConfigChange struct {
    FieldPath string      // Dot-separated path (e.g., "database.timeout")
    OldValue  interface{} // Previous value
    NewValue  interface{} // New value
}
```

### Events

| Event Type | Description | Data |
|------------|-------------|------|
| `reload.started` | Reload operation initiated | `{reloadID, trigger, timestamp}` |
| `reload.validated` | Configuration changes validated | `{reloadID, changeCount}` |
| `reload.module.started` | Module reload started | `{reloadID, module}` |
| `reload.module.completed` | Module reload completed | `{reloadID, module, duration}` |
| `reload.completed` | All modules reloaded successfully | `{reloadID, duration, changeCount}` |
| `reload.failed` | Reload operation failed | `{reloadID, error, failedModule}` |
| `reload.rolledback` | Configuration rolled back | `{reloadID, reason}` |

## Performance Considerations

1. **Reload Frequency**: Avoid reloading too frequently. Use appropriate check intervals.
2. **Change Detection**: File watching has minimal overhead. Polling should use reasonable intervals.
3. **Module Impact**: Ensure reload operations are lightweight and don't disrupt service.
4. **Caching**: The system caches configuration to avoid unnecessary reloads.

## Security

1. **Validation**: Always validate configuration changes before applying
2. **Secrets**: Use the `SecretValue` wrapper for sensitive configuration
3. **Audit**: All reload operations emit events for audit logging
4. **Permissions**: Restrict reload triggers to authorized users/systems

## Migration Guide

### From Static to Dynamic Configuration

1. Identify configuration that changes frequently
2. Add `dynamic:"true"` tags to those fields
3. Implement `Reloadable` interface in affected modules
4. Test reload behavior thoroughly
5. Enable dynamic reload in production

### Gradual Rollout

```go
// Start with specific modules
app := modular.NewApplication(
    modular.WithDynamicReload(
        modular.DynamicReloadConfig{
            EnabledModules: []string{"cache", "ratelimit"},
        },
    ),
)

// Later expand to all modules
app := modular.NewApplication(
    modular.WithDynamicReload(), // All modules
)
```