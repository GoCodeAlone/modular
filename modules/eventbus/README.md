# EventBus Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/eventbus.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/eventbus)

The EventBus Module provides a publish-subscribe messaging system for Modular applications with support for multiple concurrent engines, topic-based routing, and flexible configuration. It enables decoupled communication between components through a powerful event-driven architecture.

## Features

### Core Capabilities
- **Multi-Engine Support**: Run multiple event bus engines simultaneously (Memory, Redis, Kafka, Kinesis, Custom)
- **Topic-Based Routing**: Route events to different engines based on topic patterns
- **Synchronous & Asynchronous Processing**: Support for both immediate and background event processing
- **Wildcard Topics**: Subscribe to topic patterns like `user.*` or `analytics.*`
- **Event History & TTL**: Configurable event retention and cleanup policies
- **Worker Pool Management**: Configurable worker pools for async event processing

### Supported Engines
- **Memory**: In-process event bus using Go channels (default)
- **Redis**: Distributed messaging using Redis pub/sub
- **Kafka**: Enterprise messaging using Apache Kafka
- **Kinesis**: AWS-native streaming using Amazon Kinesis
- **Custom**: Support for custom engine implementations

### Advanced Features
- **Custom Engine Registration**: Register your own engine types
- **Configuration-Based Routing**: Route topics to engines via configuration
- **Engine-Specific Configuration**: Each engine can have its own settings
- **Metrics & Monitoring**: Built-in metrics collection (custom engines)
- **Tenant Isolation**: Support for multi-tenant applications
- **Graceful Shutdown**: Proper cleanup of all engines and subscriptions

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

### Single Engine Configuration (Legacy)

```yaml
eventbus:
  engine: memory              # Event bus engine (memory, redis, kafka, kinesis)
  maxEventQueueSize: 1000     # Maximum events to queue per topic
  defaultEventBufferSize: 10  # Default buffer size for subscription channels
  workerCount: 5              # Worker goroutines for async event processing
  eventTTL: 3600s             # TTL for events (duration)
  retentionDays: 7            # Days to retain event history
  externalBrokerURL: ""       # URL for external message broker
  externalBrokerUser: ""      # Username for authentication
  externalBrokerPassword: ""  # Password for authentication
```

### Multi-Engine Configuration

```yaml
eventbus:
  engines:
    - name: "memory-fast"
      type: "memory"
      config:
        maxEventQueueSize: 500
        defaultEventBufferSize: 10
        workerCount: 3
        retentionDays: 1
    - name: "redis-durable"
      type: "redis"
      config:
        url: "redis://localhost:6379"
        db: 0
        poolSize: 10
    - name: "kafka-analytics"
      type: "kafka"
      config:
        brokers: ["localhost:9092"]
        groupId: "eventbus-analytics"
    - name: "kinesis-stream"
      type: "kinesis"
      config:
        region: "us-east-1"
        streamName: "events-stream"
        shardCount: 2
    - name: "custom-engine"
      type: "custom"
      config:
        enableMetrics: true
        metricsInterval: "30s"
  routing:
    - topics: ["user.*", "auth.*"]
      engine: "memory-fast"
    - topics: ["analytics.*", "metrics.*"]
      engine: "kafka-analytics"  
    - topics: ["stream.*"]
      engine: "kinesis-stream"
    - topics: ["*"]  # Fallback for all other topics
      engine: "redis-durable"
```

## Usage

### Basic Event Publishing and Subscription

```go
// Get the eventbus service
var eventBus *eventbus.EventBusModule
err := app.GetService("eventbus.provider", &eventBus)
if err != nil {
    return fmt.Errorf("failed to get eventbus service: %w", err)
}

// Publish an event
err = eventBus.Publish(ctx, "user.created", userData)
if err != nil {
    return fmt.Errorf("failed to publish event: %w", err)
}

// Subscribe to events
subscription, err := eventBus.Subscribe(ctx, "user.created", func(ctx context.Context, event eventbus.Event) error {
    user := event.Payload.(UserData)
    fmt.Printf("User created: %s\n", user.Name)
    return nil
})
```

### Multi-Engine Routing

```go
// Events are automatically routed based on configured rules
eventBus.Publish(ctx, "user.login", userData)      // -> memory-fast engine
eventBus.Publish(ctx, "analytics.click", clickData) // -> kafka-analytics engine  
eventBus.Publish(ctx, "custom.event", customData)  // -> redis-durable engine (fallback)

// Check which engine handles a specific topic
router := eventBus.GetRouter()
engine := router.GetEngineForTopic("user.created")
fmt.Printf("Topic 'user.created' routes to engine: %s\n", engine)
```

### Custom Engine Registration

```go
// Register a custom engine type
eventbus.RegisterEngine("myengine", func(config map[string]interface{}) (eventbus.EventBus, error) {
    return NewMyCustomEngine(config), nil
})

// Use in configuration
engines:
  - name: "my-custom"
    type: "myengine" 
    config:
      customSetting: "value"
```

## Examples

### Multi-Engine Application

See [examples/multi-engine-eventbus/](../../examples/multi-engine-eventbus/) for a complete application demonstrating:

- Multiple concurrent engines
- Topic-based routing  
- Different processing patterns
- Engine-specific configuration
- Real-world event types

```bash
cd examples/multi-engine-eventbus
go run main.go
```

Sample output:
```
🚀 Started Multi-Engine EventBus Demo in development environment
📊 Multi-Engine EventBus Configuration:
  - memory-fast: Handles user.* and auth.* topics
  - memory-reliable: Handles analytics.*, metrics.*, and fallback topics

🔵 [MEMORY-FAST] User registered: user123 (action: register)
📈 [MEMORY-RELIABLE] Page view: /dashboard (session: sess123)
⚙️  [MEMORY-RELIABLE] System info: database - Connection established
```

## Engine Implementations

### Memory Engine (Built-in)
- Fast in-process messaging using Go channels
- Configurable worker pools and buffer sizes
- Event history and TTL support
- Perfect for single-process applications

### Redis Engine  
- Distributed messaging using Redis pub/sub
- Supports Redis authentication and connection pooling
- Wildcard subscriptions via Redis pattern matching
- Good for distributed applications with moderate throughput

### Kafka Engine
- Enterprise messaging using Apache Kafka
- Consumer group support for load balancing
- SASL authentication and SSL/TLS support
- Ideal for high-throughput, durable messaging

### Kinesis Engine
- AWS-native streaming using Amazon Kinesis
- Multiple shard support for scalability
- Automatic stream management
- Perfect for AWS-based applications with analytics needs

### Custom Engine
- Example implementation with metrics and filtering
- Demonstrates custom engine development patterns
- Includes event metrics collection and topic filtering
- Template for building specialized engines

## Testing

The module includes comprehensive BDD tests covering:

- Single and multi-engine configurations
- Topic routing and engine selection  
- Custom engine registration
- Synchronous and asynchronous processing
- Error handling and recovery
- Tenant isolation scenarios

```bash
cd modules/eventbus
go test ./... -v
```

## Migration from Single-Engine

Existing single-engine configurations continue to work unchanged. To migrate to multi-engine:

```yaml
# Before (single-engine)
eventbus:
  engine: memory
  workerCount: 5

# After (multi-engine with same behavior) 
eventbus:
  engines:
    - name: "default"
      type: "memory"
      config:
        workerCount: 5
```

## Performance Considerations

### Engine Selection Guidelines
- **Memory**: Best for high-performance, low-latency scenarios
- **Redis**: Good for distributed applications with moderate throughput  
- **Kafka**: Ideal for high-throughput, durable messaging
- **Kinesis**: Best for AWS-native applications with streaming analytics
- **Custom**: Use for specialized requirements

### Configuration Tuning
```yaml
# High-throughput configuration
eventbus:
  engines:
    - name: "high-perf"
      type: "memory" 
      config:
        maxEventQueueSize: 10000
        defaultEventBufferSize: 100
        workerCount: 20
```

## Contributing

When contributing to the eventbus module:

1. Add tests for new engine implementations
2. Update BDD scenarios for new features
3. Document configuration options thoroughly  
4. Ensure backward compatibility
5. Add examples demonstrating new capabilities

## License

This module is part of the Modular framework and follows the same license terms.