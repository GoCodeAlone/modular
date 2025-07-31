# Observer Pattern Example

This example demonstrates the Observer pattern implementation in the Modular framework. It shows how to:

- Use `ObservableApplication` for automatic event emission
- Create modules that implement the `Observer` interface
- Register observers for specific event types
- Emit custom events from modules
- Use the `EventLogger` module for structured event logging
- Handle errors gracefully in observers

## Features Demonstrated

### 1. ObservableApplication
- Automatically emits events for module/service registration and application lifecycle
- Thread-safe observer management
- Event filtering by type

### 2. EventLogger Module
- Multiple output targets (console, file, syslog)
- Configurable log levels and formats
- Event type filtering
- Async processing with buffering

### 3. Custom Observable Modules
- **UserModule**: Emits custom events for user operations
- **NotificationModule**: Observes user events and sends notifications
- **AuditModule**: Observes all events for compliance logging

### 4. Event Types
- Framework events: `module.registered`, `service.registered`, `application.started`
- Custom events: `user.created`, `user.login`

## Running the Example

### Basic Usage
```bash
go run .
```

### Generate Sample Configuration
```bash
go run . --generate-config yaml config-sample.yaml
```

### Environment Variables
You can override configuration using environment variables:
```bash
EVENTLOGGER_LOGLEVEL=DEBUG go run .
USERMODULE_MAXUSERS=50 go run .
```

## Expected Output

When you run the example, you'll see:

1. **Application startup events** logged by EventLogger
2. **Module registration events** for each registered module
3. **Service registration events** for registered services
4. **Custom user events** when users are created and log in
5. **Notification handling** by the NotificationModule
6. **Audit logging** of all events by the AuditModule
7. **Application shutdown events** during graceful shutdown

## Configuration

The example uses a comprehensive configuration that demonstrates:

- EventLogger with console output and optional file logging
- Configurable log levels and formats
- Event type filtering options
- User module configuration

## Observer Pattern Flow

1. **ObservableApplication** emits framework lifecycle events
2. **EventLogger** observes all events and logs them to configured outputs
3. **UserModule** emits custom events for business operations
4. **NotificationModule** observes user events and sends notifications
5. **AuditModule** observes all events for compliance and security

## Code Structure

- `main.go` - Application setup and coordination
- `user_module.go` - Demonstrates event emission and observation
- `notification_module.go` - Demonstrates event-driven notifications
- `audit_module.go` - Demonstrates comprehensive event auditing
- `config.yaml` - Configuration with event logging setup

## Key Learning Points

1. **Observer Registration**: How to register observers for specific event types
2. **Event Emission**: How modules can emit custom events
3. **Error Handling**: How observer errors are handled gracefully
4. **Configuration**: How to configure event logging and filtering
5. **Integration**: How the Observer pattern integrates with the existing framework

## Testing

Run the example and observe the detailed event logging that shows the Observer pattern in action. The output demonstrates:

- Real-time event processing
- Event filtering and routing
- Error handling and recovery
- Performance with async processing