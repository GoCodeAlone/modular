# Interface Compatibility Migration Guide

## v1.11.1 Application Interface Changes

The `Application` interface has been enhanced with three new methods to support the enhanced service registry functionality:

```go
// New methods added in v1.11.1
GetServicesByModule(moduleName string) []string
GetServiceEntry(serviceName string) (*ServiceRegistryEntry, bool)
GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry
```

### Migration for Mock Applications

If you have custom implementations of the `Application` interface (e.g., for testing), you'll need to add these methods:

```go
import "reflect"

type MockApplication struct {
    // ... existing fields
}

// Add these new methods to satisfy the updated Application interface
func (m *MockApplication) GetServicesByModule(moduleName string) []string {
    return []string{} // Return empty slice for mock
}

func (m *MockApplication) GetServiceEntry(serviceName string) (*ServiceRegistryEntry, bool) {
    return nil, false // Return not found for mock
}

func (m *MockApplication) GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry {
    return nil // Return empty for mock
}
```

### Migration for Application Decorators

If you have decorator patterns around the Application interface, ensure they delegate to the underlying implementation:

```go
type ApplicationDecorator struct {
    app Application
}

func (d *ApplicationDecorator) GetServicesByModule(moduleName string) []string {
    return d.app.GetServicesByModule(moduleName)
}

func (d *ApplicationDecorator) GetServiceEntry(serviceName string) (*ServiceRegistryEntry, bool) {
    return d.app.GetServiceEntry(serviceName)
}

func (d *ApplicationDecorator) GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry {
    return d.app.GetServicesByInterface(interfaceType)
}
```

## Nil Service Instance Handling

Version v1.11.1 also fixes panics that could occur when modules provide services with nil instances during interface-based dependency resolution. The framework now gracefully handles these cases:

- Services with `nil` instances are skipped during interface matching
- Nil type checking is performed before reflection operations
- Logger calls are protected against nil loggers

### Best Practices

To avoid issues with nil service instances:

1. **Validate service instances before registration:**
   ```go
   func (m *MyModule) ProvidesServices() []ServiceProvider {
       if m.serviceInstance == nil {
           return []ServiceProvider{} // Don't provide nil services
       }
       return []ServiceProvider{{
           Name: "myService",
           Instance: m.serviceInstance,
       }}
   }
   ```

2. **Use proper error handling in module initialization:**
   ```go
   func (m *MyModule) Init(app Application) error {
       if m.requiredDependency == nil {
           return fmt.Errorf("required dependency not available")
       }
       return nil
   }
   ```

3. **Test your modules with the new interface methods:**
   ```go
   func TestModuleWithEnhancedRegistry(t *testing.T) {
       app := modular.NewStdApplication(nil, logger)
       module := &MyModule{}
       app.RegisterModule(module)
       
       err := app.Init()
       require.NoError(t, err)
       
       // Test the new interface methods
       services := app.GetServicesByModule("myModule")
       // ... verify services
   }
   ```