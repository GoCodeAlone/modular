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
- **Memory**: In-process event bus using Go channels (default). Configurable delivery modes: `drop`, `block`, `timeout`.
- **Durable Memory**: In-process event bus that **never drops events**. Uses a per-subscriber FIFO queue and blocks publishers (backpressure) when the queue is full. Ideal when event loss is unacceptable within a single process.
- **Redis**: Distributed messaging using Redis pub/sub
- **Kafka**: Enterprise messaging using Apache Kafka
- **Kinesis**: AWS-native streaming using Amazon Kinesis
- **NATS**: Lightweight, high-performance messaging using NATS
- **Custom**: Support for custom engine implementations

### Advanced Features
- **Custom Engine Registration**: Register your own engine types
- **Configuration-Based Routing**: Route topics to engines via configuration
- **Engine-Specific Configuration**: Each engine can have its own settings
- **Metrics & Monitoring**: Built-in metrics collection (custom engines)
- **Tenant Isolation**: Support for multi-tenant applications
- **Graceful Shutdown**: Proper cleanup of all engines and subscriptions
 - **Delivery Stats API**: Lightweight counters for delivered vs dropped events (memory engine) aggregated per-engine and module-wide
 - **Metrics Exporters**: Prometheus collector and Datadog StatsD exporter for delivery statistics

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
    - name: "nats-messaging"
      type: "nats"
      config:
        url: "nats://localhost:4222"
        connectionName: "modular-eventbus"
        maxReconnects: 10
        reconnectWait: 2
        allowReconnect: true
        pingInterval: 20
        maxPingsOut: 2
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

### Delivery Modes & Backpressure (Memory Engine)

The in-process memory engine supports configurable delivery semantics to balance throughput, fairness, and reliability when subscriber channels become congested.

Configuration fields (per engine config.map for a memory engine):

```yaml
eventbus:
  engines:
    - name: "memory-fast"
      type: "memory"
      config:
        # Existing settings...
        workerCount: 5
        maxEventQueueSize: 1000
        defaultEventBufferSize: 32

        # New delivery / fairness controls
        deliveryMode: drop            # drop | block | timeout (default: drop)
        publishBlockTimeout: 250ms    # only used when deliveryMode: timeout
        rotateSubscriberOrder: true   # fairness rotation (default: true)
```

Modes:
- drop (default): Non-blocking send. If a subscriber channel buffer is full the event is dropped for that subscriber (other subscribers still attempted). Highest throughput, possible per-subscriber loss under bursty load.
- block: Publisher goroutine blocks until each subscriber accepts the event (or context cancelled). Provides strongest delivery at the cost of publisher backpressure; a slow subscriber stalls publishers.
- timeout: Like block but each subscriber send has an upper bound (`publishBlockTimeout`). If the timeout elapses the event is dropped for that subscriber and publishing proceeds. Reduces head-of-line blocking risk while greatly lowering starvation compared to pure drop mode.

Fairness:
- When `rotateSubscriberOrder` is true (default) the memory engine performs a deterministic rotation of the subscriber slice based on a monotonically increasing publish counter. This gives each subscription a chance to be first periodically, preventing chronic starvation when buffers are near capacity.
- When false, iteration order is the static registration order (legacy behavior) and early subscribers can dominate under sustained pressure. A light random shuffle is applied per publish as a best-effort mitigation.

Observability:
- The memory engine maintains internal delivered and dropped counters (exposed via a `Stats()` method).
- Module-level helpers expose aggregate (`eventBus.Stats()`) and per-engine (`eventBus.PerEngineStats()`) delivery counts suitable for exporting to metrics backends (Prometheus, Datadog, etc.). Example:

```go
delivered, dropped := eventBus.Stats()
perEngine := eventBus.PerEngineStats() // map[engineName]DeliveryStats
for name, s := range perEngine {
  fmt.Printf("engine=%s delivered=%d dropped=%d\n", name, s.Delivered, s.Dropped)
}
```

Test Stability Note:
Async subscriptions are processed via a worker pool so their delivered count may lag momentarily after publishers finish. When writing tests that compare sync vs async distribution, allow a short settling period (poll until async count stops increasing) and use wide fairness bounds (e.g. async within 25%–300% of sync) to avoid flakiness while still detecting pathological starvation.

Backward Compatibility:
- If you do not set any of the new fields, behavior remains equivalent to previous versions (drop mode with fairness rotation enabled by default, which improves starvation resilience without changing loss semantics).

Tuning Guidance:
- Start with `drop` in high-throughput low-criticality paths where occasional loss is acceptable.
- Use `timeout` with a modest `publishBlockTimeout` (e.g. 5-50ms) for balanced fairness and latency in mixed-speed subscriber sets.
- Reserve `block` for critical fan-out where all subscribers must process every event and you are comfortable applying backpressure to publishers.
- Use `durable-memory` when you need zero event loss within a process without the operational overhead of an external broker.

Example (balanced):
```yaml
eventbus:
  engines:
    - name: "memory-balanced"
      type: "memory"
      config:
        workerCount: 8
        defaultEventBufferSize: 64
        deliveryMode: timeout
        publishBlockTimeout: 25ms
        rotateSubscriberOrder: true
```

### Durable Memory Engine (Zero Event Loss)

The `durable-memory` engine is an in-process alternative to `memory` that **never drops events**. Instead of dropping events when a subscriber is busy, publishers block (backpressure) until the subscriber's queue has space. Memory usage is bounded by `maxDurableQueueDepth × number-of-subscribers`.

**When to use:**
- Event loss is unacceptable and the application runs in a single process
- You need the simplicity of in-process delivery with stronger guarantees than `memory`+`block` mode
- You want bounded memory without the operational overhead of Redis or Kafka

**Single-engine configuration:**
```yaml
eventbus:
  engine: durable-memory
  maxEventQueueSize: 1000      # fallback queue depth if maxDurableQueueDepth is 0
  maxDurableQueueDepth: 500    # per-subscriber queue depth (0 = use maxEventQueueSize)
```

**Multi-engine configuration (mixed with a lossy fast path):**
```yaml
eventbus:
  engines:
    - name: "fast"
      type: "memory"
      config:
        workerCount: 8
        deliveryMode: drop
    - name: "critical"
      type: "durable-memory"
      config:
        maxDurableQueueDepth: 200
  routing:
    - topics: ["payment.*", "order.*"]
      engine: "critical"
    - topics: ["*"]
      engine: "fast"
```

**Trade-offs vs `memory` + `block`:**

| | `durable-memory` | `memory` + `block` |
|---|---|---|
| Event loss | None | None |
| Backpressure unit | Per-subscriber queue | Per-subscriber channel slot |
| Queue structure | Linked list (unbounded growth before cap) | Fixed-size Go channel |
| Memory bound | `maxDurableQueueDepth` | `defaultEventBufferSize` |
| Async handler parallelism | Sequential per subscriber | Shared worker pool |

**Important note on cross-process durability:**  
`durable-memory` only protects against in-process loss (e.g. a slow subscriber). It does **not** survive process restarts or crashes. For durable-across-restarts guarantees, use Redis, Kafka, or Kinesis.

### Metrics Export (Prometheus & Datadog)

Delivery statistics (delivered vs dropped) can be exported via the built-in Prometheus Collector or a Datadog StatsD exporter.

#### Prometheus

Register the collector with your Prometheus registry (global or custom):

```go
import (
  "github.com/GoCodeAlone/modular/modules/eventbus"
  prom "github.com/prometheus/client_golang/prometheus"
  promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
  "net/http"
)

// After module start and obtaining eventBus reference
collector := eventbus.NewPrometheusCollector(eventBus, "modular_eventbus")
prom.MustRegister(collector)

http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":2112", nil)
```

Emitted metrics (Counter):
- `modular_eventbus_delivered_total{engine="_all"}` – Aggregate delivered (processed) events
- `modular_eventbus_dropped_total{engine="_all"}` – Aggregate dropped (not processed) events
- Per-engine variants with `engine="<engineName>"`

Example PromQL:
```promql
rate(modular_eventbus_delivered_total{engine!="_all"}[5m])
rate(modular_eventbus_dropped_total{engine="_all"}[5m])
```

#### Datadog (DogStatsD)

Start the exporter in a background goroutine. It periodically snapshots stats and emits gauges.

```go
import (
  "time"
  "github.com/GoCodeAlone/modular/modules/eventbus"
)

exporter, err := eventbus.NewDatadogStatsdExporter(eventBus, eventbus.DatadogExporterConfig{
  Address:           "127.0.0.1:8125", // DogStatsD agent address
  Namespace:         "modular.eventbus.",
  FlushInterval:     5 * time.Second,
  MaxPacketSize:     1432,
  IncludePerEngine:  true,
  IncludeGoroutines: true,
})
if err != nil { panic(err) }
go exporter.Run() // call exporter.Close() on shutdown
```

Emitted gauges (namespace-prefixed):
- `delivered_total` / `dropped_total` (tags: `engine:<name>` plus aggregate `engine:_all`)
- `go.goroutines` (optional) for exporter process health

Datadog query examples:
```
avg:modular.eventbus.delivered_total{engine:_all}.as_count()
top(avg:modular.eventbus.dropped_total{*} by {engine}, 5, 'mean', 'desc')
```

#### Semantics
`delivered` counts events whose handlers executed (success or failure). `dropped` counts events that could not be enqueued or processed (channel full, timeout, worker pool saturation). These sets are disjoint per subscription, so `delivered + dropped` approximates total published events actually observed by subscribers.

#### Shutdown
Always call `exporter.Close()` (Datadog) during module/application shutdown to flush final metrics.

#### Extensibility
You can build custom exporters by polling `eventBus.PerEngineStats()` periodically and forwarding the numbers to your metrics system of choice.
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
    var user UserData
    if err := event.DataAs(&user); err != nil {
        return err
    }
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

### NATS Messaging Application

See [examples/nats-eventbus/](../../examples/nats-eventbus/) for a complete NATS-based messaging application featuring:

- Two services communicating via NATS
- Publisher service generating order events
- Subscriber service processing events
- Wildcard topic subscriptions
- Async event handlers
- Docker Compose setup with NATS

```bash
cd examples/nats-eventbus
./run-demo.sh run
```

Sample output:
```
🚀 Started NATS EventBus Demo in development environment
📊 NATS EventBus Configuration:
  - NATS server: localhost:4222
  - All topics routed through NATS

📤 [PUBLISHED] order.created: ORDER-1 (amount: $100.99)
📨 [ORDER SERVICE] Processing order: ORDER-1
📨 [ANALYTICS SERVICE] Recording event: order_created
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

### NATS Engine
- Lightweight, high-performance messaging using NATS
- Support for wildcards and hierarchical subjects
- Automatic reconnection and connection management
- Ideal for microservices and real-time messaging
- Optional authentication with username/password or token
- JetStream support for persistent messaging

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
- **NATS**: Perfect for lightweight, cloud-native microservices
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