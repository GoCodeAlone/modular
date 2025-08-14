# Multi-Engine EventBus Example

This example demonstrates the enhanced eventbus module with multi-engine support, topic routing, and integration with Redis alongside in-memory engines. It shows clear event publishing and consumption patterns with graceful degradation when external services are unavailable.

## Features Demonstrated

- **Multiple Event Bus Engines**: Configure and use multiple engines simultaneously
  - **Memory engines**: Fast in-memory processing for low-latency events  
  - **Redis engine**: Distributed pub/sub messaging with Redis persistence
  - **Custom engine**: Enhanced memory engine with metrics collection
- **Topic-based Routing**: Routes different event types to appropriate engines based on topic patterns  
- **Event Consumption Visibility**: Clear indicators showing event publishing and consumption
- **Local Redis Setup**: Simple Docker setup for Redis service testing
- **Graceful Degradation**: Handles cases where Redis is unavailable with automatic fallback
- **Engine-Specific Configuration**: Demonstrates engine-specific configuration options
- **Robust Error Handling**: Graceful shutdown and error handling for production scenarios

## Architecture Overview

The example configures three engines with intelligent routing:

1. **memory-fast**: Fast in-memory engine for user and authentication events
   - Handles topics: `user.*`, `auth.*`
   - Optimized for low latency with smaller buffers and fewer workers

2. **redis-primary**: Redis engine for system, health, and notification events
   - Handles topics: `system.*`, `health.*`, `notifications.*`
   - Provides distributed pub/sub messaging with Redis persistence

3. **memory-reliable**: Custom memory engine with metrics for fallback events
   - Handles all other topics not matched by specific rules
   - Includes event metrics collection and larger buffers for reliability

## Prerequisites

- **Go 1.24+**: For running the application
- **Docker**: For running Redis locally (optional - app works without it)
- **Git**: For cloning the repository

## Quick Start

### Option 1: Use the Setup Script (Recommended)

The `run-demo.sh` script handles Redis setup automatically:

```bash
# Start Redis and run the demo
./run-demo.sh run-redis

# Or start just Redis for testing
./run-demo.sh redis

# Run without external services (graceful degradation)
./run-demo.sh app
```

### Option 2: Manual Setup

```bash
# 1. Run without external services (shows graceful degradation)
go run main.go

# 2. Or start Redis manually and then run
docker run -d -p 6379:6379 redis:alpine
go run main.go

# 3. Stop Redis when done
docker stop $(docker ps -q --filter ancestor=redis:alpine)
```

## Expected Output

### With Redis Available

When Redis is running, you'll see:

```
🚀 Started Multi-Engine EventBus Demo in development environment
📊 Multi-Engine EventBus Configuration:
  - memory-fast: Handles user.* and auth.* topics (in-memory, low latency)
  - redis-primary: Handles system.*, health.*, and notifications.* topics (Redis pub/sub, distributed)
  - memory-reliable: Handles fallback topics (in-memory with metrics)

🔍 Checking external service availability:
  ✅ Redis service is reachable on localhost:6379
  ⚠️ EventBus router is not routing to redis-primary (engine may have failed to start)

📡 Setting up event handlers (showing consumption patterns)...
✅ All event handlers configured and ready to consume events

🎯 Publishing events to different engines based on topic routing:
   📤 [PUBLISHED] = Event sent    📨 [CONSUMED] = Event received by handler

🔵 Memory-Fast Engine Events:
📤 [PUBLISHED] user.registered: user123
📨 [CONSUMED] User registered: user123 (action: register) → memory-fast engine
...
```

### Without Redis (Graceful Degradation)

When Redis is unavailable, the system gracefully falls back:

```
🔍 Checking external service availability:
  ❌ Redis service not reachable, system/health/notifications events will route to fallback
  💡 To enable Redis: docker run -d -p 6379:6379 redis:alpine
```

All events are still processed using the memory engines, demonstrating robust fault tolerance.
./run-demo.sh run

# Or start services separately
./run-demo.sh start
go run main.go

# Stop services when done
./run-demo.sh stop

# Clean up everything (including volumes)
./run-demo.sh cleanup
```

### Option 2: Manual Setup

1. **Start the external services**:
   ```bash
   docker-compose up -d
   ```

2. **Wait for services to be ready** (about 1-2 minutes):
   ```bash
   # Check Redis
   docker exec eventbus-redis redis-cli ping
   
   # Check Kafka
   docker exec eventbus-kafka kafka-topics --bootstrap-server localhost:9092 --list
   ```

3. **Run the application**:
   ```bash
   go run main.go
   ```

4. **Stop services when done**:
   ```bash
   docker-compose down
   ```

## Configuration Details

### Routing Rules

```yaml
routing:
  - topics: ["user.*", "auth.*"]
    engine: "memory-fast"
  - topics: ["analytics.*", "metrics.*"]  
    engine: "kafka-analytics"
  - topics: ["system.*", "health.*"]
    engine: "redis-durable"
  - topics: ["*"]  # Fallback rule
    engine: "memory-reliable"
```

### Engine Configurations

**Memory Fast Engine**:
```yaml
name: "memory-fast"
type: "memory"
config:
  maxEventQueueSize: 500
  defaultEventBufferSize: 10
  workerCount: 3
  retentionDays: 1
```

**Redis Engine**:
```yaml
name: "redis-durable"
type: "redis"
config:
  url: "redis://localhost:6379"
  db: 0
  poolSize: 10
```

**Kafka Engine**:
```yaml
name: "kafka-analytics"
type: "kafka"
config:
  brokers: ["localhost:9092"]
  groupId: "multi-engine-demo"
```

## Available Commands

The `run-demo.sh` script provides several useful commands:

```bash
./run-demo.sh start     # Start Redis and Kafka services
./run-demo.sh stop      # Stop the services
./run-demo.sh cleanup   # Stop services and remove volumes
./run-demo.sh run       # Start services and run the demo
./run-demo.sh app       # Run only the Go app (services must be running)
./run-demo.sh status    # Show service status
./run-demo.sh logs      # Show service logs
./run-demo.sh help      # Show detailed help
```

## Expected Output

The example will:

1. Start and configure all four engines (memory-fast, kafka-analytics, redis-durable, memory-reliable)
2. Check the availability of external services (Redis and Kafka)
3. Set up event handlers for different topic types and engines
4. Publish events to demonstrate routing to different engines
5. Show which engine processes each event type with clear labeling
6. Display active topics and subscriber counts
7. Show detailed routing information
8. Gracefully shut down all engines

## Sample Output

```
🚀 Started Multi-Engine EventBus Demo in development environment
📊 Multi-Engine EventBus Configuration:
  - memory-fast: Handles user.* and auth.* topics (in-memory, low latency)
  - kafka-analytics: Handles analytics.* and metrics.* topics (distributed, persistent)
  - redis-durable: Handles system.* and health.* topics (Redis pub/sub, persistent)
  - memory-reliable: Handles fallback topics (in-memory with metrics)

🔍 Checking external service availability:
  ✅ Redis engine configured and ready
  ✅ Kafka engine configured and ready

🎯 Publishing events to different engines based on topic routing:

🔵 [MEMORY-FAST] User registered: user123 (action: register)
🔵 [MEMORY-FAST] User login: user456 at 15:04:05
🔴 [MEMORY-FAST] Auth failed for user: user789
📈 [KAFKA-ANALYTICS] Page view: /dashboard (session: sess123)
📈 [KAFKA-ANALYTICS] Click event: click on /dashboard
📊 [KAFKA-ANALYTICS] CPU usage metric received
⚙️  [REDIS-DURABLE] System info: database - Connection established
🏥 [REDIS-DURABLE] Health check: loadbalancer - All endpoints healthy
🔄 [MEMORY-RELIABLE] Fallback event processed

⏳ Processing events...

📋 Event Bus Routing Information:
  user.registered -> memory-fast
  user.login -> memory-fast
  auth.failed -> memory-fast
  analytics.pageview -> kafka-analytics
  analytics.click -> kafka-analytics
  metrics.cpu_usage -> kafka-analytics
  system.health -> redis-durable
  health.check -> redis-durable
  random.topic -> memory-reliable

📊 Active Topics and Subscriber Counts:
  user.registered: 1 subscribers
  user.login: 1 subscribers
  auth.failed: 1 subscribers
  analytics.pageview: 1 subscribers
  analytics.click: 1 subscribers
  metrics.cpu_usage: 1 subscribers
  system.health: 1 subscribers
  health.check: 1 subscribers
  fallback.test: 1 subscribers

🛑 Shutting down...
✅ Application shutdown complete
```

## Troubleshooting

### Services Not Available

If you see messages like "❌ Redis engine not available" or "❌ Kafka engine not available":

1. **Check if Docker is running**: `docker --version`
2. **Start the services**: `./run-demo.sh start`
3. **Check service status**: `./run-demo.sh status`
4. **View service logs**: `./run-demo.sh logs`

### Common Issues

**Port conflicts**: If ports 6379 (Redis) or 9092 (Kafka) are in use:
```bash
# Check what's using the ports
netstat -tlnp | grep :6379
netstat -tlnp | grep :9092

# Stop conflicting services or modify docker-compose.yml ports
```

**Docker Compose version**: The script auto-detects `docker compose` vs `docker-compose`:
```bash
# Check your version
docker compose version  # Newer
# or
docker-compose version   # Older
```

**Services taking too long to start**: 
- Redis usually starts in ~10 seconds
- Kafka can take 30-60 seconds due to Zookeeper dependency
- Use `./run-demo.sh logs` to monitor startup progress

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
eventBus.Publish(ctx, "user.login", userData)         // -> memory-fast
eventBus.Publish(ctx, "analytics.click", clickData)   // -> kafka-analytics
eventBus.Publish(ctx, "system.health", healthData)    // -> redis-durable
eventBus.Publish(ctx, "custom.event", customData)     // -> memory-reliable (fallback)
```

### Engine-Specific Configuration
```go
config := eventbus.EngineConfig{
    Name: "my-kafka",
    Type: "kafka",
    Config: map[string]interface{}{
        "brokers": []string{"localhost:9092"},
        "groupId": "my-consumer-group",
    },
}
```

### Service Discovery and Health Checks
```go
// Check which engine will handle a topic
router := eventBus.GetRouter()
engine := router.GetEngineForTopic("analytics.click")  // "kafka-analytics"

// Get active topics and subscriber counts
activeTopics := eventBus.Topics()
for _, topic := range activeTopics {
    count := eventBus.SubscriberCount(topic)
    fmt.Printf("%s: %d subscribers\n", topic, count)
}
```

## Architecture Benefits

- **Scalability**: Different engines can be optimized for different workloads
- **Reliability**: Critical events can use more reliable engines while fast events use optimized ones  
- **Isolation**: Different types of events are processed independently
- **Flexibility**: Easy to add new engines or change routing without code changes
- **Monitoring**: Per-engine metrics and logging for better observability
- **Development**: Complete local development environment with real services
- **Production Ready**: Same configuration works in production with external service endpoints

## Production Considerations

### Redis Configuration
```yaml
redis:
  url: "redis://prod-redis:6379"
  password: "${REDIS_PASSWORD}"
  poolSize: 20
  db: 1
```

### Kafka Configuration
```yaml
kafka:
  brokers: ["kafka1:9092", "kafka2:9092", "kafka3:9092"]
  groupId: "production-consumers"
  security:
    protocol: "SASL_SSL"
    username: "${KAFKA_USERNAME}"
    password: "${KAFKA_PASSWORD}"
```

### High Availability Setup
- Use Redis Cluster or Sentinel for Redis HA
- Use Kafka clusters with multiple brokers and replicas
- Configure appropriate retention policies
- Set up monitoring and alerting
- Use circuit breakers for external service failures

## Development Workflow

1. **Local Development**: Use Docker Compose for local Redis/Kafka
2. **Testing**: Unit tests with mock engines, integration tests with real services
3. **Staging**: Connect to shared staging Redis/Kafka clusters
4. **Production**: Use managed services (ElastiCache, MSK, Confluent, etc.)

## Advanced Usage

### Custom Engine Implementation
```go
type MyCustomEngine struct {
    // Implementation
}

func NewMyCustomEngine(config map[string]interface{}) (EventBus, error) {
    // Factory function
}

// Register the engine
eventbus.RegisterEngine("mycustom", NewMyCustomEngine)
```

### Multi-Tenant Routing
```go
routing:
  - topics: ["tenant1.*"]
    engine: "redis-tenant1"
  - topics: ["tenant2.*"]
    engine: "kafka-tenant2"
```

### Environment-Specific Configuration
```go
// Development: Use in-memory engines
// Staging: Use shared Redis/Kafka
// Production: Use managed services with authentication
```

## Next Steps

Try modifying the example to:

1. **Add custom authentication** for Redis and Kafka
2. **Implement custom event filtering** in engines  
3. **Add tenant-aware routing** for multi-tenant applications
4. **Experiment with different partition strategies** in Kafka
5. **Add monitoring and metrics collection** for all engines
6. **Create custom engines** for other message brokers (NATS, RabbitMQ, etc.)
7. **Add event replay and dead letter queue functionality**