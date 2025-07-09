# Environment Variable Catalog and Feeder Integration

This document describes how the Modular framework's enhanced configuration system handles environment variables and manages feeder precedence.

## Overview

The Modular framework uses a unified **Environment Catalog** system that combines environment variables from multiple sources:
- Operating System environment variables
- `.env` file variables
- Dynamically set variables

This allows all environment-based feeders (EnvFeeder, AffixedEnvFeeder, InstanceAwareEnvFeeder, TenantAffixedEnvFeeder) to access variables from both OS environment and .env files with proper precedence handling.

## Environment Catalog System

### EnvCatalog Architecture

The `EnvCatalog` provides:
- **Unified Access**: Single interface for all environment variables
- **Source Tracking**: Tracks whether variables come from OS env or .env files
- **Precedence Management**: OS environment always takes precedence over .env files
- **Thread Safety**: Concurrent access safe with mutex protection

### Variable Precedence

1. **OS Environment Variables** (highest precedence)
2. **.env File Variables** (lower precedence)

When the same variable exists in both sources, the OS environment value is used.

### Global Catalog

- Single global catalog instance shared across all env-based feeders
- Initialized once and reused for performance
- Can be reset for testing scenarios

## Feeder Types and Integration

### File-Based Feeders
These feeders read from configuration files and populate structs directly:

1. **YamlFeeder**: Reads YAML files, supports nested structures
2. **JSONFeeder**: Reads JSON files, handles complex object hierarchies  
3. **TomlFeeder**: Reads TOML files, supports all TOML data types
4. **DotEnvFeeder**: Special hybrid - loads .env into catalog AND populates structs

### Environment-Based Feeders
These feeders read from the unified Environment Catalog:

1. **EnvFeeder**: Basic env var lookup using struct field `env` tags
2. **AffixedEnvFeeder**: Adds prefix/suffix to env variable names
3. **InstanceAwareEnvFeeder**: Handles instance-specific configurations
4. **TenantAffixedEnvFeeder**: Combines tenant-aware and affixed behavior

### DotEnvFeeder Behavior

The `DotEnvFeeder` has dual behavior:
1. **Catalog Population**: Loads .env variables into the global catalog for other env feeders
2. **Direct Population**: Populates config structs using catalog (respects OS env precedence)

This allows other env-based feeders to access .env variables while maintaining proper precedence.

## Field-Level Tracking

All feeders support comprehensive field-level tracking that records:

- **Field Path**: Complete field path (e.g., "Database.Connections.primary.DSN")
- **Field Type**: Data type of the field
- **Feeder Type**: Which feeder populated the field
- **Source Type**: Source category (env, yaml, json, toml, dotenv)
- **Source Key**: The actual key used (e.g., "DB_PRIMARY_DSN")
- **Value**: The value that was set
- **Search Keys**: All keys that were searched
- **Found Key**: The key that was actually found
- **Instance Info**: For instance-aware feeders

### Tracking Usage

```go
tracker := NewDefaultFieldTracker()
feeder.SetFieldTracker(tracker)

// After feeding
populations := tracker.GetFieldPopulations()
for _, pop := range populations {
    fmt.Printf("Field %s set to %v from %s\n", 
        pop.FieldPath, pop.Value, pop.SourceKey)
}
```

## Feeder Evaluation Order and Precedence

### Recommended Order

When using multiple feeders, the typical order is:

1. **File-based feeders** (YAML/JSON/TOML) - set base configuration
2. **DotEnvFeeder** - load .env variables into catalog  
3. **Environment-based feeders** - override with env-specific values

### Precedence Rules

**Within the same feeder type**: Last feeder wins (overwrites previous values)

**Between feeder types**: Order of execution determines precedence

**For environment variables**: OS environment always beats .env files

### Example Multi-Feeder Setup

```go
config := modular.NewConfig()

// Base configuration from YAML
config.AddFeeder(feeders.NewYamlFeeder("config.yaml"))

// Load .env into catalog for other env feeders
config.AddFeeder(feeders.NewDotEnvFeeder(".env"))

// Environment-based overrides
config.AddFeeder(feeders.NewEnvFeeder())
config.AddFeeder(feeders.NewAffixedEnvFeeder("APP_", "_PROD"))

// Feed the configuration
err := config.Feed(&appConfig)
```

### Precedence Flow

```
YAML values → DotEnv values → OS Env values → Affixed Env values
   (base)    →  (if not in OS) →  (override)  →   (final override)
```

## Environment Variable Naming Patterns

### EnvFeeder
Uses env tags directly: `env:"DATABASE_URL"`

### AffixedEnvFeeder  
Constructs: `PREFIX__ENVTAG__SUFFIX`
- Example: `PROD_` + `HOST` + `_ENV` = `PROD__HOST__ENV`
- Uses double underscores between components

### InstanceAwareEnvFeeder
Constructs: `MODULE_INSTANCE_FIELD`
- Example: `DB_PRIMARY_DSN`, `DB_SECONDARY_DSN`

### TenantAffixedEnvFeeder
Combines tenant ID with affixed pattern:
- Example: `APP_TENANT123__CONFIG__PROD`

## Error Handling

The system uses static error definitions to comply with linting rules:

```go
var (
    ErrDotEnvInvalidStructureType = errors.New("expected pointer to struct")
    ErrJSONCannotConvert         = errors.New("cannot convert value to field type")
    // ... more specific errors
)
```

Errors are wrapped with context using `fmt.Errorf("%w: %s", baseError, context)`.

## Verbose Debug Logging

All feeders support verbose debug logging for troubleshooting:

```go
feeder.SetVerboseDebug(true, logger)
```

Debug output includes:
- Environment variable lookups and results
- Field processing steps
- Type conversion attempts
- Source tracking information
- Error details with context

## Best Practices

### Configuration Setup
1. Use file-based feeders for base configuration
2. Use DotEnvFeeder to load .env files for local development
3. Use env-based feeders for deployment-specific overrides
4. Set up field tracking for debugging and audit trails

### Environment Variable Management
1. Use consistent naming patterns
2. Document env var precedence in your application
3. Test with both OS env and .env file scenarios
4. Use verbose debugging during development

### Error Handling
1. Always check feeder errors during configuration loading
2. Use field tracking to identify configuration sources
3. Validate required fields after feeding
4. Provide clear error messages for missing configuration

## Testing Considerations

### Test Isolation
```go
// Reset catalog between tests
feeders.ResetGlobalEnvCatalog()

// Use t.Setenv for test environment variables
t.Setenv("TEST_VAR", "test_value")
```

### Multi-Feeder Testing
Test various combinations of feeders to ensure proper precedence handling.

### Field Tracking Validation
Verify that field tracking correctly reports source information for debugging and audit purposes.
