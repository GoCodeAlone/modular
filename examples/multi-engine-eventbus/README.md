# Multi-Engine EventBus Example

This example demonstrates the enhanced eventbus module with multi-engine support, topic routing, and integration with the eventlogger module.

## Features Demonstrated

- **Multiple Event Bus Engines**: Shows how to configure and use multiple engines simultaneously
- **Topic-based Routing**: Routes different types of events to different engines based on topic patterns
- **Custom Engine Configuration**: Demonstrates engine-specific configuration settings
- **Event Logging Integration**: Uses the eventlogger module to log events across engines
- **Synchronous and Asynchronous Processing**: Shows both sync and async event handlers

## Configuration

The example configures two engines:

1. **memory-fast**: Fast in-memory engine for user and authentication events
   - Handles topics: `user.*`, `auth.*`
   - Optimized for low latency with smaller buffers and fewer workers

2. **memory-reliable**: Custom memory engine with metrics for analytics and system events
   - Handles topics: `analytics.*`, `metrics.*`, and fallback for all other topics
   - Includes event metrics collection and larger buffers for reliability

## Routing Rules

```yaml
routing:
  - topics: ["user.*", "auth.*"]
    engine: "memory-fast"
  - topics: ["analytics.*", "metrics.*"]  
    engine: "memory-reliable"
  - topics: ["*"]  # Fallback rule
    engine: "memory-reliable"
```

## Running the Example

```bash
cd examples/multi-engine-eventbus
go run main.go
```

## Expected Output

The example will:

1. Initialize both engines and show the routing configuration
2. Set up event handlers for different topic types
3. Publish events to demonstrate routing to different engines
4. Show which engine processes each event type
5. Display active topics and subscriber counts
6. Gracefully shut down all engines

## Sample Output

```
🚀 Started Multi-Engine EventBus Demo in development environment
📊 Multi-Engine EventBus Configuration:
  - memory-fast: Handles user.* and auth.* topics
  - memory-reliable: Handles analytics.*, metrics.*, and fallback topics

🎯 Publishing events to different engines based on topic routing:

🔵 [MEMORY-FAST] User registered: user123 (action: register)
🔵 [MEMORY-FAST] User login: user456 at 15:04:05
🔴 [MEMORY-FAST] Auth failed for user: user789
📈 [MEMORY-RELIABLE] Page view: /dashboard (session: sess123)
📈 [MEMORY-RELIABLE] Click event: click on /dashboard
⚙️  [MEMORY-RELIABLE] System info: database - Connection established

⏳ Processing events...

📋 Event Bus Routing Information:
  user.registered -> memory-fast
  user.login -> memory-fast
  auth.failed -> memory-fast
  analytics.pageview -> memory-reliable
  analytics.click -> memory-reliable
  system.health -> memory-reliable
  random.topic -> memory-reliable

📊 Active Topics and Subscriber Counts:
  user.registered: 1 subscribers
  user.login: 1 subscribers
  auth.failed: 1 subscribers
  analytics.pageview: 1 subscribers
  analytics.click: 1 subscribers
  system.health: 1 subscribers

🛑 Shutting down...
✅ Application shutdown complete
```

## Key Concepts

### Engine Registration
```go
// Engines are registered automatically at startup
// Custom engines can be registered with:
eventbus.RegisterEngine("myengine", MyEngineFactory)
```

### Topic Routing
```go
// Events are automatically routed based on configured rules
eventBus.Publish(ctx, "user.login", userData)      // -> memory-fast
eventBus.Publish(ctx, "analytics.click", clickData) // -> memory-reliable  
eventBus.Publish(ctx, "custom.event", customData)  // -> memory-reliable (fallback)
```

### Engine-Specific Configuration
```go
config := eventbus.EngineConfig{
    Name: "my-engine",
    Type: "custom",
    Config: map[string]interface{}{
        "enableMetrics": true,
        "bufferSize":   1000,
    },
}
```

## Architecture Benefits

- **Scalability**: Different engines can be optimized for different workloads
- **Reliability**: Critical events can use more reliable engines while fast events use optimized ones  
- **Isolation**: Different types of events are processed independently
- **Flexibility**: Easy to add new engines or change routing without code changes
- **Monitoring**: Per-engine metrics and logging for better observability

## Next Steps

Try modifying the example to:

1. Add Redis or Kafka engines (requires external services)
2. Implement custom event filtering in engines
3. Add tenant-aware routing for multi-tenant applications
4. Experiment with different routing patterns and priorities