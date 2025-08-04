# EventBus Demo

A comprehensive demonstration of the EventBus module's pub/sub messaging capabilities.

## Features

- **Event Publishing**: Publish events to topics via REST API
- **Event Subscription**: Automatic subscription to user and order events
- **Message History**: View received messages through the API
- **Topic Management**: List active topics and subscriber counts
- **Statistics**: View real-time statistics about the event bus
- **Async Processing**: Demonstrates both sync and async event handling

## Quick Start

1. **Start the application:**
   ```bash
   go run main.go
   ```

2. **Check health:**
   ```bash
   curl http://localhost:8080/health
   ```

3. **Publish an event:**
   ```bash
   curl -X POST http://localhost:8080/api/eventbus/publish \
     -H "Content-Type: application/json" \
     -d '{
       "topic": "user.created",
       "content": "New user John Doe registered",
       "metadata": {
         "user_id": "12345",
         "source": "registration-service"
       }
     }'
   ```

4. **View received messages:**
   ```bash
   curl http://localhost:8080/api/eventbus/messages
   ```

## API Endpoints

### Event Management

- **POST /api/eventbus/publish** - Publish an event
  ```json
  {
    "topic": "user.created",
    "content": "Event payload content",
    "metadata": {
      "key": "value"
    }
  }
  ```

- **GET /api/eventbus/messages** - Get received messages
  - Query params: `limit` (default: 100)

- **DELETE /api/eventbus/messages** - Clear message history

### Information

- **GET /api/eventbus/topics** - List active topics and subscriber counts
- **GET /api/eventbus/stats** - Get event bus statistics
- **GET /health** - Health check endpoint

## Event Patterns

The demo automatically subscribes to these event patterns:

### User Events (Synchronous)
- **user.created** - New user registration
- **user.updated** - User profile updates
- **user.deleted** - User account deletion

### Order Events (Asynchronous)
- **order.placed** - New order created
- **order.confirmed** - Order confirmation
- **order.shipped** - Order shipment
- **order.delivered** - Order delivery

## Example Usage

### Publish Different Event Types

```bash
# User registration event
curl -X POST http://localhost:8080/api/eventbus/publish \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "user.created",
    "content": "User Alice registered",
    "metadata": {"user_id": "alice123", "email": "alice@example.com"}
  }'

# Order placed event  
curl -X POST http://localhost:8080/api/eventbus/publish \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "order.placed",
    "content": "Order #1001 placed",
    "metadata": {"order_id": "1001", "amount": "99.99"}
  }'

# Custom business event
curl -X POST http://localhost:8080/api/eventbus/publish \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "inventory.low",
    "content": "Product inventory below threshold",
    "metadata": {"product_id": "prod-456", "current_stock": "5"}
  }'
```

### View Results

```bash
# Check what topics are active
curl http://localhost:8080/api/eventbus/topics

# View recent messages
curl http://localhost:8080/api/eventbus/messages?limit=10

# Get statistics
curl http://localhost:8080/api/eventbus/stats
```

## Event Bus Features Demonstrated

1. **Topic-based Routing**: Events are routed to subscribers based on topic patterns
2. **Sync vs Async**: User events are processed synchronously, order events asynchronously
3. **Metadata Support**: Events can carry additional metadata for context
4. **Wildcard Subscriptions**: Using patterns like `user.*` to catch all user events
5. **Message History**: Track all events that have been processed
6. **Topic Management**: Monitor active topics and subscriber counts

## Configuration

The EventBus module is configured in `config.yaml`:

```yaml
eventbus:
  engine: memory                    # Event bus engine type
  maxEventQueueSize: 1000          # Max events to queue per topic
  defaultEventBufferSize: 10       # Default buffer size for subscriptions
  workerCount: 5                   # Worker goroutines for async processing
  eventTTL: 3600                   # TTL for events in seconds
  retentionDays: 7                 # Days to retain event history
```

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │────│   REST API      │────│   EventBus      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                       │
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Sync Handler  │────│ User Events     │
                       └─────────────────┘    └─────────────────┘
                                                       │
                       ┌─────────────────┐    ┌─────────────────┐
                       │  Async Handler  │────│ Order Events    │
                       └─────────────────┘    └─────────────────┘
```

## Learning Objectives

This demo teaches:

- How to integrate EventBus module with Modular applications
- Publishing events programmatically and via API
- Subscribing to events with sync and async handlers
- Using topic patterns for flexible event routing
- Managing event metadata and history
- Monitoring event bus performance and statistics

## Production Considerations

- Use appropriate worker pool sizes for your load
- Implement proper error handling in event handlers
- Consider event persistence for critical systems
- Monitor memory usage with high event volumes
- Use structured logging for event processing
- Implement circuit breakers for external dependencies

## Next Steps

- Integrate with external message brokers (Redis, Kafka)
- Add event schema validation
- Implement event replay capabilities
- Add distributed event processing
- Create event-driven microservices architecture