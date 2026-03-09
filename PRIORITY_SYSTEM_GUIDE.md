# Feeder Priority Control System

## Overview

The Modular framework now supports explicit priority control for configuration feeders, allowing you to precisely control which configuration sources override others. This solves common issues like test isolation where environment variables would unintentionally override explicit test configurations.

## Quick Start

### Basic Usage

```go
import "github.com/GoCodeAlone/modular/feeders"

// Add feeders with priority control
config.AddFeeder(feeders.NewYamlFeeder("config.yaml").WithPriority(50))
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))
```

**Key Concept:** Higher priority values = applied later = override lower priority feeders

## Common Patterns

### 1. Test Isolation Pattern

**Problem:** Environment variables from the host system override explicit test configuration.

**Solution:** Give test configuration higher priority than environment variables.

```go
func TestWithIsolation(t *testing.T) {
    // Host may have SDK_KEY="production-key"
    t.Setenv("SDK_KEY", "host-value")

    // Test wants specific configuration
    yamlPath := createTestYAML(t, `sdkKey: "test-value"`)

    config := modular.NewConfig()
    config.AddFeeder(feeders.NewEnvFeeder().WithPriority(50))       // Lower priority
    config.AddFeeder(feeders.NewYamlFeeder(yamlPath).WithPriority(100)) // Higher priority
    config.AddStructKey("_main", &cfg)
    config.Feed()

    // Test gets explicit YAML value, not environment variable
    assert.Equal(t, "test-value", cfg.SDKKey)
}
```

### 2. Production Override Pattern

**Problem:** Need environment variables to override default configuration files.

**Solution:** Give environment variables higher priority than config files.

```go
// Production application setup
config := modular.NewConfig()

// Base configuration (lower priority)
config.AddFeeder(feeders.NewYamlFeeder("config.yaml").WithPriority(50))

// Environment overrides (higher priority)
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))
config.AddFeeder(feeders.NewAffixedEnvFeeder("APP_", "_PROD").WithPriority(100))

config.AddStructKey("_main", &appConfig)
config.Feed()
```

### 3. Layered Configuration Pattern

**Problem:** Multiple configuration sources with clear precedence hierarchy.

**Solution:** Use different priority levels for each layer.

```go
config := modular.NewConfig()

// Layer 1: Defaults (lowest priority)
config.AddFeeder(feeders.NewYamlFeeder("defaults.yaml").WithPriority(10))

// Layer 2: Environment-specific configuration
config.AddFeeder(feeders.NewYamlFeeder("config-prod.yaml").WithPriority(50))

// Layer 3: Local .env file overrides
config.AddFeeder(feeders.NewDotEnvFeeder(".env").WithPriority(75))

// Layer 4: OS environment variables (highest priority)
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))

config.AddStructKey("_main", &appConfig)
config.Feed()
```

## Priority Guidelines

### Recommended Priority Ranges

| Range | Purpose | Examples |
|-------|---------|----------|
| 0-50 | Base/default configuration | Default YAML files, built-in defaults |
| 51-99 | Environment-specific configuration | .env files, environment-specific configs |
| 100+ | Runtime overrides | OS environment variables, command-line flags |

### Best Practices

1. **Use consistent priority ranges** across your application
2. **Document your priority scheme** in application documentation
3. **Leave gaps between priorities** (e.g., 10, 50, 100) for future additions
4. **Group related feeders** at the same priority level
5. **Higher priority for more specific configuration** (env vars > files > defaults)

## Default Behavior

**Without explicit priorities:**
- All feeders default to priority 0
- Sequential order is preserved (last feeder wins)
- Maintains backward compatibility with existing code

```go
// These are equivalent:
config.AddFeeder(feeders.NewYamlFeeder("config.yaml"))
config.AddFeeder(feeders.NewEnvFeeder())

// Same as:
config.AddFeeder(feeders.NewYamlFeeder("config.yaml").WithPriority(0))
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(0))
```

## Technical Details

### How Priority Works

1. Before feeding configuration, all feeders are sorted by priority (ascending)
2. Feeders with equal priority maintain their original order (stable sort)
3. Feeders are applied in sorted order (lowest to highest priority)
4. Later feeders override values set by earlier feeders

### Priority Interface

```go
type PrioritizedFeeder interface {
    Feeder
    Priority() int
}
```

All built-in feeders implement this interface:
- `YamlFeeder`
- `JSONFeeder`
- `TomlFeeder`
- `EnvFeeder`
- `DotEnvFeeder`
- `AffixedEnvFeeder`
- `TenantAffixedEnvFeeder`

### Verbose Debugging

Enable verbose debugging to see feeder application order:

```go
config := modular.NewConfig()
config.SetVerboseDebug(true, logger)
config.AddFeeder(feeders.NewYamlFeeder("config.yaml").WithPriority(50))
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))
config.Feed()

// Output shows:
// Feeder order: index=0, type=*feeders.YamlFeeder, priority=50
// Feeder order: index=1, type=*feeders.EnvFeeder, priority=100
```

## Migration Guide

### Updating Existing Code

**No changes required!** The priority system is fully backward compatible.

**Optional upgrade to explicit priorities:**

Before:
```go
config.AddFeeder(feeders.NewYamlFeeder("config.yaml"))
config.AddFeeder(feeders.NewEnvFeeder())
// Relies on order: Env overrides YAML
```

After:
```go
config.AddFeeder(feeders.NewYamlFeeder("config.yaml").WithPriority(50))
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))
// Explicit: Env overrides YAML because of higher priority
```

### Fixing Test Isolation Issues

Before (broken - env vars override test config):
```go
func TestMyFeature(t *testing.T) {
    // Host environment interferes with test
    config.AddFeeder(feeders.NewYamlFeeder("test-config.yaml"))
    config.AddFeeder(feeders.NewEnvFeeder())
    // Problem: Env vars override test config
}
```

After (fixed - test config overrides env vars):
```go
func TestMyFeature(t *testing.T) {
    // Test configuration takes precedence
    config.AddFeeder(feeders.NewEnvFeeder().WithPriority(50))
    config.AddFeeder(feeders.NewYamlFeeder("test-config.yaml").WithPriority(100))
    // Solution: Test config overrides env vars
}
```

## Examples

See these files for complete examples:
- `feeder_priority_test.go` - Comprehensive test scenarios
- `issue_reproduction_test.go` - Original issue demonstration
- `examples/basic-app/main.go` - Usage comments
- `feeders/DOCUMENTATION.md` - Complete documentation

## FAQ

**Q: What happens if I don't specify priority?**  
A: Default priority is 0. Original sequential behavior is preserved.

**Q: Can I use negative priorities?**  
A: Yes, but it's not recommended. Use 0+ for clarity.

**Q: What if two feeders have the same priority?**  
A: Original order is preserved (stable sort). Later feeder wins.

**Q: Does this work with module-specific configuration?**  
A: Yes, priority applies to all configuration feeding, including module configs.

**Q: Can I change priority after creating a feeder?**
A: Yes, call `WithPriority()` again. It returns the feeder for chaining.

## Troubleshooting

### Configuration not applying as expected

1. **Enable verbose debugging:**
   ```go
   config.SetVerboseDebug(true, logger)
   ```

2. **Check feeder order in logs:**
   ```
   Feeder order: index=0, type=*feeders.YamlFeeder, priority=50
   Feeder order: index=1, type=*feeders.EnvFeeder, priority=100
   ```

3. **Verify priorities:**
   - Higher priority = applied later = overrides
   - Lower priority = applied earlier = can be overridden

### Tests failing with environment variables

**Symptom:** Tests pass locally but fail in CI with different environment variables.

**Solution:** Use priority control to make test config override environment:
```go
config.AddFeeder(feeders.NewEnvFeeder().WithPriority(50))
config.AddFeeder(feeders.NewYamlFeeder("test-config.yaml").WithPriority(100))
```

## Version History

- **v1.12.0**: Added priority control system with `WithPriority()` method
- Full backward compatibility maintained

## See Also

- [feeders/DOCUMENTATION.md](feeders/DOCUMENTATION.md) - Complete feeder documentation
- [CONFIG_PROVIDERS.md](CONFIG_PROVIDERS.md) - Configuration provider patterns
- [examples/](examples/) - Working examples
