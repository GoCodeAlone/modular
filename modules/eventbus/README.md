# EventBus Module

The EventBus Module provides a publish-subscribe messaging system for Modular applications. It enables decoupled communication between components through a flexible event-driven architecture.

## Features

- In-memory event publishing and subscription
- Support for both synchronous and asynchronous event handling
- Topic-based routing
- Event history tracking
- Configurable worker pool for asynchronous event processing
- Extensible design with support for external message brokers

## Installation

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/eventbus"
)

// Register the eventbus module with your Modular application
app.RegisterModule(eventbus.NewModule())
```

## Configuration

The eventbus module can be configured using the following options:

```yaml
eventbus:
  engine: memory              # Event bus engine (memory, redis, kafka)
  maxEventQueueSize: 1000     # Maximum events to queue per topic
  defaultEventBufferSize: 10  # Default buffer size for subscription channels
  workerCount: 5              # Worker goroutines for async event processing
  eventTTL: 3600              # TTL for events in seconds (1 hour)
  retentionDays: 7            # Days to retain event history
  externalBrokerURL: ""       # URL for external message broker (if used)
  externalBrokerUser: ""      # Username for external message broker (if used)
  externalBrokerPassword: ""  # Password for external message broker (if used)
```

## Usage

### Accessing the EventBus Service

```go
// In your module's Init function
func (m *MyModule) Init(app modular.Application) error {
    var eventBusService *eventbus.EventBusModule
    err := app.GetService("eventbus.provider", &eventBusService)
    if err != nil {
        return fmt.Errorf("failed to get event bus service: %w", err)
    }
    
    // Now you can use the event bus service
    m.eventBus = eventBusService
    return nil
}
```

### Using Interface-Based Service Matching

```go
// Define the service dependency
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "eventbus",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*eventbus.EventBus)(nil)).Elem(),
        },
    }
}

// Access the service in your constructor
func (m *MyModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        eventBusService := services["eventbus"].(eventbus.EventBus)
        return &MyModule{eventBus: eventBusService}, nil
    }
}
```

### Publishing Events

```go
// Publish a simple event
err := eventBusService.Publish(ctx, "user.created", user)
if err != nil {
    // Handle error
}

// Publish an event with metadata
metadata := map[string]interface{}{
    "source": "user-service",
    "version": "1.0",
}

event := eventbus.Event{
    Topic:    "user.created",
    Payload:  user,
    Metadata: metadata,
}

err = eventBusService.Publish(ctx, event)
if err != nil {
    // Handle error
}
```

### Subscribing to Events

```go
// Synchronous subscription
subscription, err := eventBusService.Subscribe(ctx, "user.created", func(ctx context.Context, event eventbus.Event) error {
    user := event.Payload.(User)
    fmt.Printf("User created: %s\n", user.Name)
    return nil
})

if err != nil {
    // Handle error
}

// Asynchronous subscription (handler runs in a worker goroutine)
asyncSub, err := eventBusService.SubscribeAsync(ctx, "user.created", func(ctx context.Context, event eventbus.Event) error {
    // This function is executed asynchronously
    user := event.Payload.(User)
    time.Sleep(1 * time.Second) // Simulating work
    fmt.Printf("Processed user asynchronously: %s\n", user.Name)
    return nil
})

// Unsubscribe when done
defer eventBusService.Unsubscribe(ctx, subscription)
defer eventBusService.Unsubscribe(ctx, asyncSub)
```

### Working with Topics

```go
// List all active topics
topics := eventBusService.Topics()
fmt.Println("Active topics:", topics)

// Get subscriber count for a topic
count := eventBusService.SubscriberCount("user.created")
fmt.Printf("Subscribers for 'user.created': %d\n", count)
```

## Event Handling Best Practices

1. **Keep Handlers Lightweight**: Event handlers should be quick and efficient, especially for synchronous subscriptions

2. **Error Handling**: Always handle errors in your event handlers, especially for async handlers

3. **Topic Organization**: Use hierarchical topics like "domain.event.action" for better organization

4. **Type Safety**: Consider defining type-safe wrappers around the event bus for specific event types

5. **Context Usage**: Use the provided context to implement cancellation and timeouts

## Implementation Notes

- The in-memory event bus uses channels to distribute events to subscribers
- Asynchronous handlers are executed in a worker pool to limit concurrency
- Event history is retained based on the configured retention period
- The module is extensible to support external message brokers in the future

## Testing

The eventbus module includes tests for module initialization, configuration, and lifecycle management.