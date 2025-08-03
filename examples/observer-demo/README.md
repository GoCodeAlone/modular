# Observer Demo Example

This example demonstrates the new decorator pattern and builder API for the Modular framework, showcasing:

1. **Builder Pattern**: Using functional options to construct applications
2. **Decorator Pattern**: Applying decorators for tenant awareness and observability
3. **Observer Pattern**: Event-driven communication using CloudEvents
4. **Event Logger Module**: Automatic logging of all application events

## Features Demonstrated

### New Builder API
```go
app, err := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithConfigDecorators(
        modular.InstanceAwareConfig(),
        modular.TenantAwareConfigDecorator(tenantLoader),
    ),
    modular.WithTenantAware(tenantLoader),
    modular.WithObserver(customEventObserver),
    modular.WithModules(
        eventlogger.NewModule(),
        &DemoModule{},
    ),
)
```

### Decorator Pattern
- **TenantAwareDecorator**: Adds tenant resolution and multi-tenant capabilities
- **ObservableDecorator**: Emits CloudEvents for application lifecycle events
- **ConfigDecorators**: Instance-aware and tenant-aware configuration decoration

### Observer Pattern Integration
- **Functional Observers**: Simple function-based event handlers
- **Module Observers**: Modules can register as observers for specific events
- **Event Logger**: Automatic logging of all CloudEvents in the system

## Running the Example

```bash
cd examples/observer-demo
go run main.go
```

## Expected Output

The application will:
1. Start with tenant resolution (demo-tenant-1, demo-tenant-2)
2. Initialize and start the EventLogger module
3. Emit lifecycle events (before/after init, start, stop) 
4. Log all events via the EventLogger module (visible in console output)
5. Display custom observer notifications with event details
6. Demonstrate module-to-module event communication
7. Show both functional observers and module observers working together

## Migration from Old API

### Before (Old API)
```go
cfg := &AppConfig{}
configProvider := modular.NewStdConfigProvider(cfg)
app := modular.NewStdApplication(configProvider, logger)
app.RegisterModule(NewDatabaseModule())
app.RegisterModule(NewAPIModule())
app.Run()
```

### After (New Builder API)
```go
app := modular.NewApplication(
    modular.WithLogger(logger),
    modular.WithConfigProvider(configProvider),
    modular.WithTenantAware(tenantLoader),
    modular.WithObserver(observerFunc),
    modular.WithModules(
        NewDatabaseModule(),
        NewAPIModule(),
        eventlogger.NewEventLoggerModule(),
    ),
)
app.Run()
```

## Event Flow

1. **Application Lifecycle**: Start/stop events automatically emitted
2. **Module Registration**: Each module registration emits events
3. **Custom Events**: Modules can emit their own CloudEvents
4. **Observer Chain**: Multiple observers can handle the same events
5. **Event Logging**: All events are logged by the EventLogger module