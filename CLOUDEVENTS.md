# CloudEvents Integration for Modular Framework

This document describes the CloudEvents integration added to the Modular framework's Observer pattern, providing standardized event format and better interoperability with external systems.

## Overview

The CloudEvents integration enhances the existing Observer pattern by adding support for the [CloudEvents](https://cloudevents.io) specification. This provides:

- **Standardized Event Format**: Consistent metadata and structure across all events
- **Better Interoperability**: Compatible with external systems and cloud services
- **Transport Protocol Independence**: Events can be transmitted via HTTP, gRPC, AMQP, etc.
- **Built-in Validation**: Automatic validation and serialization through the CloudEvents SDK
- **Future-Proofing**: Ready for service extraction and microservices architecture

## Key Features

### Dual Event Support
- **Traditional ObserverEvents**: Backward compatibility with existing code
- **CloudEvents**: Standardized format for modern applications
- **Automatic Conversion**: Seamless conversion between event formats

### Enhanced Observer Pattern
- **CloudEventObserver**: Extended observer interface for CloudEvents
- **CloudEventSubject**: Extended subject interface for CloudEvent emission
- **FunctionalCloudEventObserver**: Convenience implementation using function callbacks

### Framework Integration
- **ObservableApplication**: Emits both traditional and CloudEvents for all lifecycle events
- **EventLogger Module**: Enhanced to log both event types with CloudEvent metadata
- **Comprehensive Examples**: Working demonstrations of CloudEvents usage

## CloudEvents Structure

CloudEvents provide a standardized structure with required and optional fields:

```go
// CloudEvent example
event := modular.NewCloudEvent(
    "com.example.user.created",    // Type (required)
    "user-service",                 // Source (required)
    userData,                       // Data (optional)
    map[string]interface{}{         // Extensions/Metadata (optional)
        "tenantId": "tenant-123",
        "version": "1.0",
    },
)

// Additional CloudEvent attributes
event.SetSubject("user-123")
event.SetTime(time.Now())
// ID and SpecVersion are set automatically
```

### CloudEvent Type Naming Convention

CloudEvent types follow a reverse domain naming convention:

```go
// Framework lifecycle events
const (
    CloudEventTypeModuleRegistered     = "com.modular.module.registered"
    CloudEventTypeServiceRegistered    = "com.modular.service.registered"
    CloudEventTypeApplicationStarted   = "com.modular.application.started"
    // ... more types
)

// Application-specific events
const (
    UserCreated = "com.myapp.user.created"
    OrderPlaced = "com.myapp.order.placed"
    PaymentProcessed = "com.myapp.payment.processed"
)
```

## Usage Examples

### Basic CloudEvent Emission

```go
// Create observable application
app := modular.NewObservableApplication(configProvider, logger)

// Emit a CloudEvent
event := modular.NewCloudEvent(
    "com.example.user.created",
    "user-service",
    map[string]interface{}{
        "userID": "user-123",
        "email": "user@example.com",
    },
    nil,
)

err := app.NotifyCloudEventObservers(context.Background(), event)
```

### CloudEvent Observer

```go
// Observer that handles both traditional and CloudEvents
observer := modular.NewFunctionalCloudEventObserver(
    "my-observer",
    // Traditional event handler
    func(ctx context.Context, event modular.ObserverEvent) error {
        log.Printf("Traditional event: %s", event.Type)
        return nil
    },
    // CloudEvent handler
    func(ctx context.Context, event cloudevents.Event) error {
        log.Printf("CloudEvent: %s (ID: %s)", event.Type(), event.ID())
        return nil
    },
)

app.RegisterObserver(observer)
```

### Module with CloudEvent Support

```go
type MyModule struct {
    app    modular.Application
    logger modular.Logger
}

// Implement ObservableModule for full CloudEvent support
func (m *MyModule) EmitCloudEvent(ctx context.Context, event cloudevents.Event) error {
    if observableApp, ok := m.app.(*modular.ObservableApplication); ok {
        return observableApp.NotifyCloudEventObservers(ctx, event)
    }
    return fmt.Errorf("application does not support CloudEvents")
}

// Register as observer for specific CloudEvent types
func (m *MyModule) RegisterObservers(subject modular.Subject) error {
    return subject.RegisterObserver(m, 
        modular.CloudEventTypeUserCreated,
        modular.CloudEventTypeOrderPlaced,
    )
}

// Handle CloudEvents
func (m *MyModule) OnCloudEvent(ctx context.Context, event cloudevents.Event) error {
    switch event.Type() {
    case modular.CloudEventTypeUserCreated:
        return m.handleUserCreated(ctx, event)
    case modular.CloudEventTypeOrderPlaced:
        return m.handleOrderPlaced(ctx, event)
    }
    return nil
}
```

## Event Conversion

### ObserverEvent to CloudEvent

```go
observerEvent := modular.ObserverEvent{
    Type:      "user.created",
    Source:    "user-service",
    Data:      userData,
    Metadata:  map[string]interface{}{"version": "1.0"},
    Timestamp: time.Now(),
}

cloudEvent := modular.ToCloudEvent(observerEvent)
// Results in CloudEvent with:
// - Type: "user.created"
// - Source: "user-service"
// - Data: userData (as JSON)
// - Extensions: {"version": "1.0"}
// - Time: observerEvent.Timestamp
// - ID: auto-generated
// - SpecVersion: "1.0"
```

### CloudEvent to ObserverEvent

```go
cloudEvent := modular.NewCloudEvent("user.created", "user-service", userData, nil)
observerEvent := modular.FromCloudEvent(cloudEvent)
// Results in ObserverEvent with converted fields
```

## EventLogger Integration

The EventLogger module automatically handles both event types:

```yaml
eventlogger:
  enabled: true
  logLevel: INFO
  format: json
  outputTargets:
    - type: console
      level: INFO
      format: structured
    - type: file
      level: DEBUG
      format: json
      file:
        path: /var/log/events.log
```

CloudEvents are logged with additional metadata:

```json
{
  "timestamp": "2024-01-15T10:30:15Z",
  "level": "INFO",
  "type": "com.modular.module.registered",
  "source": "application",
  "data": {"moduleName": "auth", "moduleType": "AuthModule"},
  "metadata": {
    "cloudevent_id": "20240115103015.123456",
    "cloudevent_specversion": "1.0",
    "cloudevent_subject": "module-auth"
  }
}
```

## Configuration

### Application Configuration

```yaml
# Use ObservableApplication for CloudEvent support
application:
  type: observable

# Configure modules for CloudEvent handling
myModule:
  enableCloudEvents: true
  eventNamespace: "com.myapp"
```

### Module Configuration

```go
type ModuleConfig struct {
    EnableCloudEvents bool   `yaml:"enableCloudEvents" default:"true" desc:"Enable CloudEvent emission"`
    EventNamespace    string `yaml:"eventNamespace" default:"com.myapp" desc:"CloudEvent type namespace"`
}
```

## Best Practices

### Event Type Naming

```go
// Good: Reverse domain notation
"com.mycompany.myapp.user.created"
"com.mycompany.myapp.order.placed"

// Avoid: Generic names
"user.created"
"event"
```

### Event Data Structure

```go
// Good: Structured data
event := modular.NewCloudEvent(
    "com.myapp.user.created",
    "user-service",
    map[string]interface{}{
        "userID": "user-123",
        "email": "user@example.com",
        "createdAt": time.Now().Unix(),
    },
    map[string]interface{}{
        "version": "1.0",
        "tenantId": "tenant-123",
    },
)

// Avoid: Unstructured data
event.SetData("raw string data")
```

### Error Handling

```go
// Validate CloudEvents before emission
if err := modular.ValidateCloudEvent(event); err != nil {
    return fmt.Errorf("invalid CloudEvent: %w", err)
}

// Handle observer errors gracefully
func (o *MyObserver) OnCloudEvent(ctx context.Context, event cloudevents.Event) error {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("CloudEvent observer panic: %v", r)
        }
    }()
    
    // Process event...
    return nil
}
```

## Performance Considerations

### Async Processing
- CloudEvent notification is asynchronous and non-blocking
- Events are processed in separate goroutines
- Buffer overflow is handled gracefully

### Memory Usage
- CloudEvents include additional metadata fields
- Consider event data size for high-volume applications
- Use event filtering to reduce processing overhead

### Network Overhead
- CloudEvents are larger than traditional ObserverEvents
- JSON serialization adds overhead for network transport
- Consider binary encoding for high-performance scenarios

## Migration Guide

### From Traditional Observer Pattern

1. **Application**: Replace `NewStdApplication` with `NewObservableApplication`
2. **Observers**: Implement `CloudEventObserver` interface alongside `Observer`
3. **Event Emission**: Add CloudEvent emission alongside traditional events
4. **Configuration**: Update EventLogger configuration for CloudEvent metadata

### Gradual Migration

```go
// Phase 1: Dual emission (backward compatible)
app.NotifyObservers(ctx, observerEvent)                    // Traditional
app.NotifyCloudEventObservers(ctx, cloudEvent)             // CloudEvent

// Phase 2: CloudEvent only (after observer migration)
app.NotifyCloudEventObservers(ctx, cloudEvent)             // CloudEvent only
```

## Testing CloudEvents

```go
func TestCloudEventEmission(t *testing.T) {
    app := modular.NewObservableApplication(mockConfig, mockLogger)
    
    events := []cloudevents.Event{}
    observer := modular.NewFunctionalCloudEventObserver(
        "test-observer",
        nil, // No traditional handler
        func(ctx context.Context, event cloudevents.Event) error {
            events = append(events, event)
            return nil
        },
    )
    
    app.RegisterObserver(observer)
    
    testEvent := modular.NewCloudEvent("test.event", "test", nil, nil)
    err := app.NotifyCloudEventObservers(context.Background(), testEvent)
    
    assert.NoError(t, err)
    assert.Len(t, events, 1)
    assert.Equal(t, "test.event", events[0].Type())
}
```

## CloudEvents SDK Integration

The implementation uses the official CloudEvents Go SDK:

```go
import cloudevents "github.com/cloudevents/sdk-go/v2"

// Access full CloudEvents SDK features
event := cloudevents.NewEvent()
event.SetSource("my-service")
event.SetType("com.example.data.created")
event.SetData(cloudevents.ApplicationJSON, data)

// Use CloudEvents client for HTTP transport
client, err := cloudevents.NewClientHTTP()
if err != nil {
    log.Fatal(err)
}

result := client.Send(context.Background(), event)
```

## Future Enhancements

### Planned Features
- **HTTP Transport**: Direct CloudEvent HTTP emission
- **NATS Integration**: CloudEvent streaming via NATS
- **Schema Registry**: Event schema validation and versioning
- **Event Sourcing**: CloudEvent store for event sourcing patterns

### Extension Points
- **Custom Transports**: Implement CloudEvent transport protocols
- **Event Transformation**: CloudEvent data transformation pipelines
- **Event Routing**: Content-based CloudEvent routing
- **Monitoring**: CloudEvent metrics and tracing integration

## Conclusion

The CloudEvents integration enhances the Modular framework's Observer pattern with industry-standard event format while maintaining full backward compatibility. This provides a solid foundation for building event-driven applications that can scale from monoliths to distributed systems.

For questions or contributions, see the main [README](../README.md) and [DOCUMENTATION](../DOCUMENTATION.md).