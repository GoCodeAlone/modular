# EventLogger Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/eventlogger.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/eventlogger)

The EventLogger Module provides structured logging capabilities for Observer pattern events in Modular applications. It acts as an Observer that can be registered with any Subject to log events to various output targets including console, files, and syslog.

## Features

- **Multiple Output Targets**: Support for console, file, and syslog outputs
- **Configurable Log Levels**: DEBUG, INFO, WARN, ERROR with per-target configuration
- **Multiple Output Formats**: Text, JSON, and structured formats
- **Event Type Filtering**: Log only specific event types
- **Async Processing**: Non-blocking event processing with buffering
- **Log Rotation**: Automatic file rotation for file outputs
- **Error Handling**: Graceful handling of output target failures
- **Observer Pattern Integration**: Seamless integration with ObservableApplication

## Installation

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/eventlogger"
)

// Register the eventlogger module with your Modular application
app.RegisterModule(eventlogger.NewModule())
```

## Configuration

The eventlogger module can be configured using the following options:

```yaml
eventlogger:
  enabled: true                    # Enable/disable event logging
  logLevel: INFO                   # Minimum log level (DEBUG, INFO, WARN, ERROR)
  format: structured               # Default output format (text, json, structured)
  bufferSize: 100                  # Event buffer size for async processing
  flushInterval: 5s                # How often to flush buffered events
  includeMetadata: true            # Include event metadata in logs
  includeStackTrace: false         # Include stack traces for error events
  eventTypeFilters:                # Optional: Only log specific event types
    - module.registered
    - service.registered
    - application.started
  outputTargets:
    - type: console                # Console output
      level: INFO
      format: structured
      console:
        useColor: true
        timestamps: true
    - type: file                   # File output with rotation
      level: DEBUG
      format: json
      file:
        path: /var/log/modular-events.log
        maxSize: 100               # MB
        maxBackups: 5
        maxAge: 30                 # days
        compress: true
    - type: syslog                 # Syslog output
      level: WARN
      format: text
      syslog:
        network: unix
        address: ""
        tag: modular
        facility: user
```

## Usage

### Basic Usage with ObservableApplication

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/eventlogger"
)

func main() {
    // Create application with observer support
    app := modular.NewObservableApplication(configProvider, logger)

    // Register event logger module
    app.RegisterModule(eventlogger.NewModule())

    // Initialize application - event logger will auto-register as observer
    if err := app.Init(); err != nil {
        log.Fatal(err)
    }

    // Now all application events will be logged
    app.RegisterModule(&MyModule{})  // Logged as module.registered event
    app.Start()                      // Logged as application.started event
}
```

### Manual Observer Registration

```go
// Get the event logger service
var eventLogger *eventlogger.EventLoggerModule
err := app.GetService("eventlogger.observer", &eventLogger)
if err != nil {
    log.Fatal(err)
}

// Register with any subject for specific event types
err = subject.RegisterObserver(eventLogger, "user.created", "order.placed")
if err != nil {
    log.Fatal(err)
}
```

### Event Type Filtering

```go
// Configure to only log specific event types
config := &eventlogger.EventLoggerConfig{
    EventTypeFilters: []string{
        "module.registered",
        "service.registered", 
        "application.started",
        "application.failed",
    },
}
```

## Output Formats

### Text Format
Human-readable single-line format:
```
2024-01-15 10:30:15 INFO [module.registered] application Module 'auth' registered (type=AuthModule)
```

### JSON Format
Machine-readable JSON format:
```json
{"timestamp":"2024-01-15T10:30:15Z","level":"INFO","type":"module.registered","source":"application","data":{"moduleName":"auth","moduleType":"AuthModule"},"metadata":{}}
```

### Structured Format
Detailed multi-line structured format:
```
[2024-01-15 10:30:15] INFO module.registered
  Source: application
  Data: map[moduleName:auth moduleType:AuthModule]
  Metadata: map[]
```

## Output Targets

### Console Output
Outputs to stdout with optional color coding and timestamps:

```yaml
outputTargets:
  - type: console
    level: INFO
    format: structured
    console:
      useColor: true      # ANSI color codes for log levels
      timestamps: true    # Include timestamps in output
```

### File Output
Outputs to files with automatic rotation:

```yaml
outputTargets:
  - type: file
    level: DEBUG
    format: json
    file:
      path: /var/log/events.log
      maxSize: 100        # MB before rotation
      maxBackups: 5       # Number of backup files to keep
      maxAge: 30          # Days to keep files
      compress: true      # Compress rotated files
```

### Syslog Output
Outputs to system syslog:

```yaml
outputTargets:
  - type: syslog
    level: WARN
    format: text
    syslog:
      network: unix       # unix, tcp, udp
      address: ""         # For tcp/udp: "localhost:514"
      tag: modular        # Syslog tag
      facility: user      # Syslog facility
```

## Event Level Mapping

The module automatically maps event types to appropriate log levels:

- **ERROR**: `application.failed`, `module.failed`
- **WARN**: Custom warning events
- **INFO**: `module.registered`, `service.registered`, `application.started`, etc.
- **DEBUG**: `config.loaded`, `config.validated`

## Performance Considerations

- **Async Processing**: Events are processed asynchronously to avoid blocking the application
- **Buffering**: Events are buffered in memory before writing to reduce I/O overhead
- **Error Isolation**: Failures in one output target don't affect others
- **Graceful Degradation**: Buffer overflow results in dropped events with warnings

## Error Handling

The module handles various error conditions gracefully:

- **Output Target Failures**: Logged but don't stop other targets
- **Buffer Overflow**: Oldest events are dropped with warnings
- **Configuration Errors**: Reported during module initialization
- **Observer Errors**: Logged but don't interrupt event flow

## Integration with Existing EventBus

The EventLogger module complements the existing EventBus module:

- **EventBus**: Provides pub/sub messaging between modules
- **EventLogger**: Provides structured logging of Observer pattern events
- **Use Together**: EventBus for inter-module communication, EventLogger for audit trails

## Testing

The module includes comprehensive tests:

```bash
cd modules/eventlogger
go test ./... -v
```

## Implementation Notes

- Uses Go's `log/syslog` package for syslog support
- File rotation could be enhanced with external libraries like `lumberjack`
- Async processing uses buffered channels and worker goroutines
- Thread-safe implementation supports concurrent event logging
- Implements the Observer interface for seamless integration