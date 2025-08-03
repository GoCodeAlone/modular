# Migration Guide: From Standard API to Builder Pattern

This guide helps you migrate from the traditional Modular framework API to the new decorator pattern and builder API introduced in v2.0.

## Overview of Changes

The framework has been enhanced with a new builder pattern that provides:

1. **Decorator Pattern**: Composable application decorators for cross-cutting concerns
2. **Functional Options**: Clean builder API using functional options
3. **Enhanced Observer Pattern**: Integrated CloudEvents-based event system
4. **Tenant-Aware Applications**: Built-in multi-tenancy support
5. **Configuration Decorators**: Chainable configuration enhancement

## Quick Migration Examples

### Basic Application

**Before (v1.x)**:
```go
cfg := &AppConfig{}
configProvider := modular.NewStdConfigProvider(cfg)
app := modular.NewStdApplication(configProvider, logger)
app.RegisterModule(&DatabaseModule{})
app.RegisterModule(&APIModule{})
app.Run()
```

**After (v2.x)**:
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(modular.NewStdConfigProvider(&AppConfig{})),
    modular.WithModules(
        &DatabaseModule{},
        &APIModule{},
    ),
)
if err != nil {
    logger.Error("Failed to create application", "error", err)
    os.Exit(1)
}
app.Run()
```

### Multi-Tenant Application

**Before (v1.x)**:
```go
// Required manual setup of tenant service and configuration
tenantService := modular.NewStandardTenantService(logger)
app.RegisterService("tenantService", tenantService)
// Manual tenant registration and configuration...
```

**After (v2.x)**:
```go
tenantLoader := &MyTenantLoader{}
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithTenantAware(tenantLoader),
    modular.WithConfigDecorators(
        modular.InstanceAwareConfig(),
        modular.TenantAwareConfigDecorator(tenantLoader),
    ),
    modular.WithModules(modules...),
)
```

### Observable Application

**Before (v1.x)**:
```go
// Required manual setup of ObservableApplication
app := modular.NewObservableApplication(configProvider, logger)
// Manual observer registration...
```

**After (v2.x)**:
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithObserver(myObserverFunc),
    modular.WithModules(modules...),
)
```

## Detailed Migration Steps

### Step 1: Update Application Creation

Replace `NewStdApplication` calls with the new `NewApplication` builder:

1. **Identify**: Find all `modular.NewStdApplication()` calls
2. **Replace**: Convert to `modular.NewApplication()` with options
3. **Move modules**: Convert `app.RegisterModule()` calls to `modular.WithModules()`

### Step 2: Handle Error Returns

The new builder API returns an error, so handle it appropriately:

```go
// Old: No error handling needed
app := modular.NewStdApplication(configProvider, logger)

// New: Handle potential errors
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
)
if err != nil {
    // Handle error appropriately
}
```

### Step 3: Migrate Multi-Tenant Applications

If you were using tenant functionality:

1. **Create TenantLoader**: Implement the `TenantLoader` interface
2. **Add tenant option**: Use `WithTenantAware(loader)`
3. **Add config decorators**: Use `WithConfigDecorators()` for tenant-aware configuration

### Step 4: Add Observer Functionality

For applications that need event handling:

```go
func myEventObserver(ctx context.Context, event cloudevents.Event) error {
    log.Printf("Received event: %s from %s", event.Type(), event.Source())
    return nil
}

app, err := modular.NewApplication(
    // ... other options
    modular.WithObserver(myEventObserver),
)
```

## New Functional Options

### Core Options

- `WithLogger(logger)` - Sets the application logger (required)
- `WithConfigProvider(provider)` - Sets the main configuration provider
- `WithModules(modules...)` - Registers multiple modules at once

### Decorator Options

- `WithTenantAware(loader)` - Adds tenant-aware capabilities
- `WithObserver(observers...)` - Adds event observers
- `WithConfigDecorators(decorators...)` - Adds configuration decorators

### Configuration Decorators

- `InstanceAwareConfig()` - Enables instance-aware configuration
- `TenantAwareConfigDecorator(loader)` - Enables tenant-aware configuration

## Benefits of Migration

### 1. Cleaner Code
- Single call to create fully configured applications
- Explicit dependency declaration
- Functional composition

### 2. Better Error Handling
- Early validation of configuration
- Clear error messages for missing dependencies

### 3. Enhanced Functionality
- Built-in observer pattern with CloudEvents
- Automatic tenant resolution
- Composable configuration decoration

### 4. Future Compatibility
- Decorator pattern enables easy extension
- Builder pattern allows adding new options without breaking changes

## Backward Compatibility

The old API remains available for backward compatibility:

- `NewStdApplication()` continues to work
- `NewObservableApplication()` continues to work
- Existing module interfaces remain unchanged

However, new features and optimizations will be added to the builder API.

## Common Patterns

### Pattern 1: Service-Heavy Application
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithModules(
        &DatabaseModule{},
        &CacheModule{},
        &APIModule{},
        &AuthModule{},
    ),
)
```

### Pattern 2: Multi-Tenant SaaS Application
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithTenantAware(tenantLoader),
    modular.WithObserver(auditEventObserver),
    modular.WithConfigDecorators(
        modular.InstanceAwareConfig(),
        modular.TenantAwareConfigDecorator(tenantLoader),
    ),
    modular.WithModules(
        &TenantModule{},
        &DatabaseModule{},
        &APIModule{},
    ),
)
```

### Pattern 3: Event-Driven Microservice
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithObserver(
        eventLogger,
        metricsCollector,
        alertingObserver,
    ),
    modular.WithModules(
        &EventProcessorModule{},
        &DatabaseModule{},
    ),
)
```

## Testing with New API

Update your tests to use the builder API:

```go
func TestMyApplication(t *testing.T) {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    
    app, err := modular.NewApplication(
        modular.WithLogger(logger),
        modular.WithConfigProvider(modular.NewStdConfigProvider(&TestConfig{})),
        modular.WithModules(&TestModule{}),
    )
    
    require.NoError(t, err)
    require.NotNil(t, app)
    
    // Test application behavior...
}
```

## Troubleshooting

### Common Issues

1. **ErrLoggerNotSet**: Ensure you include `WithLogger()` option
2. **Module registration order**: Use dependency interfaces for proper ordering
3. **Configuration not found**: Verify config provider is set before decorators

### Debugging

The builder provides better error messages for common configuration issues. Enable debug logging to see the construction process:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

## Next Steps

1. **Update your applications** one at a time using this guide
2. **Test thoroughly** to ensure functionality remains the same
3. **Add new features** like observers and tenant awareness as needed
4. **Review examples** in the `examples/` directory for inspiration

The new builder API provides a solid foundation for building scalable, maintainable applications with the Modular framework.