// Package eventlogger provides structured logging capabilities for Observer pattern events.
//
// This module acts as an Observer that can be registered with any Subject (like ObservableApplication)
// to log events to various output targets including console, files, and syslog.
//
// # Features
//
// The eventlogger module offers the following capabilities:
//   - Multiple output targets (console, file, syslog)
//   - Configurable log levels and formats
//   - Event type filtering
//   - Async processing with buffering
//   - Log rotation for file outputs
//   - Structured logging with metadata
//   - Error handling and recovery
//
// # Configuration
//
// The module can be configured through the EventLoggerConfig structure:
//
//	config := &EventLoggerConfig{
//	    Enabled:     true,
//	    LogLevel:    "INFO",
//	    Format:      "structured",
//	    BufferSize:  100,
//	    OutputTargets: []OutputTargetConfig{
//	        {
//	            Type: "console",
//	            Level: "INFO",
//	            Console: &ConsoleTargetConfig{
//	                UseColor: true,
//	                Timestamps: true,
//	            },
//	        },
//	        {
//	            Type: "file",
//	            Level: "DEBUG",
//	            File: &FileTargetConfig{
//	                Path: "/var/log/modular-events.log",
//	                MaxSize: 100,
//	                MaxBackups: 5,
//	                Compress: true,
//	            },
//	        },
//	    },
//	}
//
// # Usage Examples
//
// Basic usage with ObservableApplication:
//
//	// Create application with observer support
//	app := modular.NewObservableApplication(configProvider, logger)
//
//	// Register event logger module
//	eventLogger := eventlogger.NewModule()
//	app.RegisterModule(eventLogger)
//
//	// Initialize application (event logger will auto-register as observer)
//	app.Init()
//
//	// Now all application events will be logged according to configuration
//	app.RegisterModule(&MyModule{})  // This will be logged
//	app.Start()                      // This will be logged
//
// Manual observer registration:
//
//	// Get the event logger service
//	var logger *eventlogger.EventLoggerModule
//	err := app.GetService("eventlogger.observer", &logger)
//
//	// Register with any subject
//	err = subject.RegisterObserver(logger, "user.created", "order.placed")
//
// Event type filtering:
//
//	config := &EventLoggerConfig{
//	    EventTypeFilters: []string{
//	        "module.registered",
//	        "service.registered",
//	        "application.started",
//	    },
//	}
//
// # Output Formats
//
// The module supports different output formats:
//
// **Text Format**: Human-readable format
//
//	2024-01-15 10:30:15 INFO [module.registered] Module 'auth' registered (type=AuthModule)
//
// **JSON Format**: Machine-readable JSON
//
//	{"timestamp":"2024-01-15T10:30:15Z","level":"INFO","type":"module.registered","source":"application","data":{"moduleName":"auth","moduleType":"AuthModule"}}
//
// **Structured Format**: Detailed structured format
//
//	[2024-01-15 10:30:15] INFO module.registered
//	  Source: application
//	  Data:
//	    moduleName: auth
//	    moduleType: AuthModule
//	  Metadata: {}
//
// # Error Handling
//
// The event logger handles errors gracefully:
//   - Output target failures don't stop other targets
//   - Buffer overflow is handled by dropping oldest events
//   - Invalid events are logged as errors
//   - Configuration errors are reported during initialization
package eventlogger

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// ModuleName is the unique identifier for the eventlogger module.
const ModuleName = "eventlogger"

// ServiceName is the name of the service provided by this module.
const ServiceName = "eventlogger.observer"

// EventLoggerModule provides structured logging for Observer pattern events.
// It implements both Observer and CloudEventObserver interfaces to receive events
// and log them to configured output targets. Supports both traditional ObserverEvents
// and CloudEvents for standardized event handling.
type EventLoggerModule struct {
	name      string
	config    *EventLoggerConfig
	logger    modular.Logger
	outputs   []OutputTarget
	eventChan chan cloudevents.Event
	stopChan  chan struct{}
	wg        sync.WaitGroup
	started   bool
	mutex     sync.RWMutex
}

// NewModule creates a new instance of the event logger module.
// This is the primary constructor for the eventlogger module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(eventlogger.NewModule())
func NewModule() modular.Module {
	return &EventLoggerModule{
		name: ModuleName,
	}
}

// Name returns the unique identifier for this module.
func (m *EventLoggerModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure.
func (m *EventLoggerModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &EventLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		Format:            "structured",
		BufferSize:        100,
		FlushInterval:     "5s",
		IncludeMetadata:   true,
		IncludeStackTrace: false,
		OutputTargets: []OutputTargetConfig{
			{
				Type:   "console",
				Level:  "INFO",
				Format: "structured",
				Console: &ConsoleTargetConfig{
					UseColor:   true,
					Timestamps: true,
				},
			},
		},
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the eventlogger module with the application context.
func (m *EventLoggerModule) Init(app modular.Application) error {
	// Retrieve the registered config section
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*EventLoggerConfig)
	m.logger = app.Logger()

	// Initialize output targets
	m.outputs = make([]OutputTarget, 0, len(m.config.OutputTargets))
	for i, targetConfig := range m.config.OutputTargets {
		output, err := NewOutputTarget(targetConfig, m.logger)
		if err != nil {
			return fmt.Errorf("failed to create output target %d: %w", i, err)
		}
		m.outputs = append(m.outputs, output)
	}

	// Initialize channels
	m.eventChan = make(chan cloudevents.Event, m.config.BufferSize)
	m.stopChan = make(chan struct{})

	m.logger.Info("Event logger module initialized", "targets", len(m.outputs))
	return nil
}

// Start starts the event logger processing.
func (m *EventLoggerModule) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.started {
		return nil
	}

	if !m.config.Enabled {
		m.logger.Info("Event logger is disabled, skipping start")
		return nil
	}

	// Start output targets
	for _, output := range m.outputs {
		if err := output.Start(ctx); err != nil {
			return fmt.Errorf("failed to start output target: %w", err)
		}
	}

	// Start event processing goroutine
	m.wg.Add(1)
	go m.processEvents()

	m.started = true
	m.logger.Info("Event logger started")
	return nil
}

// Stop stops the event logger processing.
func (m *EventLoggerModule) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.started {
		return nil
	}

	// Signal stop
	close(m.stopChan)

	// Wait for processing to finish
	m.wg.Wait()

	// Stop output targets
	for _, output := range m.outputs {
		if err := output.Stop(ctx); err != nil {
			m.logger.Error("Failed to stop output target", "error", err)
		}
	}

	m.started = false
	m.logger.Info("Event logger stopped")
	return nil
}

// Dependencies returns the names of modules this module depends on.
func (m *EventLoggerModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module.
func (m *EventLoggerModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Event logger observer for structured event logging",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module.
func (m *EventLoggerModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module.
func (m *EventLoggerModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// RegisterObservers implements the ObservableModule interface to auto-register
// with the application as an observer.
func (m *EventLoggerModule) RegisterObservers(subject modular.Subject) error {
	if !m.config.Enabled {
		m.logger.Info("Event logger is disabled, skipping observer registration")
		return nil
	}

	// Register for all events or filtered events
	if len(m.config.EventTypeFilters) == 0 {
		err := subject.RegisterObserver(m)
		if err != nil {
			return fmt.Errorf("failed to register event logger as observer: %w", err)
		}
		m.logger.Info("Event logger registered as observer for all events")
	} else {
		err := subject.RegisterObserver(m, m.config.EventTypeFilters...)
		if err != nil {
			return fmt.Errorf("failed to register event logger as observer: %w", err)
		}
		m.logger.Info("Event logger registered as observer for filtered events", "filters", m.config.EventTypeFilters)
	}

	return nil
}

// EmitEvent allows the module to emit its own events (not implemented for logger).
func (m *EventLoggerModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	return ErrLoggerDoesNotEmitEvents
}

// OnEvent implements the Observer interface to receive and log CloudEvents.
func (m *EventLoggerModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	m.mutex.RLock()
	started := m.started
	m.mutex.RUnlock()

	if !started {
		return ErrLoggerNotStarted
	}

	// Try to send event to processing channel
	select {
	case m.eventChan <- event:
		return nil
	default:
		// Buffer is full, drop event and log warning
		m.logger.Warn("Event buffer full, dropping event", "eventType", event.Type())
		return ErrEventBufferFull
	}
}

// ObserverID returns the unique identifier for this observer.
func (m *EventLoggerModule) ObserverID() string {
	return ModuleName
}

// processEvents processes events from both event channels.
func (m *EventLoggerModule) processEvents() {
	defer m.wg.Done()

	flushInterval, _ := time.ParseDuration(m.config.FlushInterval)
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case event := <-m.eventChan:
			m.logEvent(event)

		case <-flushTicker.C:
			m.flushOutputs()

		case <-m.stopChan:
			// Process remaining events
			for {
				select {
				case event := <-m.eventChan:
					m.logEvent(event)
				default:
					m.flushOutputs()
					return
				}
			}
		}
	}
}

// logEvent logs a CloudEvent to all configured output targets.
func (m *EventLoggerModule) logEvent(event cloudevents.Event) {
	// Check if event should be logged based on level and filters
	if !m.shouldLogEvent(event) {
		return
	}

	// Extract data from CloudEvent
	var data interface{}
	if event.Data() != nil {
		// Try to unmarshal JSON data
		if err := event.DataAs(&data); err != nil {
			// Fallback to raw data
			data = event.Data()
		}
	}

	// Extract metadata from CloudEvent extensions
	metadata := make(map[string]interface{})
	for key, value := range event.Extensions() {
		metadata[key] = value
	}

	// Create log entry
	entry := &LogEntry{
		Timestamp: event.Time(),
		Level:     m.getEventLevel(event),
		Type:      event.Type(),
		Source:    event.Source(),
		Data:      data,
		Metadata:  metadata,
	}

	// Add CloudEvent specific metadata
	entry.Metadata["cloudevent_id"] = event.ID()
	entry.Metadata["cloudevent_specversion"] = event.SpecVersion()
	if event.Subject() != "" {
		entry.Metadata["cloudevent_subject"] = event.Subject()
	}

	// Send to all output targets
	for _, output := range m.outputs {
		if err := output.WriteEvent(entry); err != nil {
			m.logger.Error("Failed to write event to output target", "error", err, "eventType", event.Type())
		}
	}
}

// shouldLogEvent determines if an event should be logged based on configuration.
func (m *EventLoggerModule) shouldLogEvent(event cloudevents.Event) bool {
	// Check event type filters
	if len(m.config.EventTypeFilters) > 0 {
		found := false
		for _, filter := range m.config.EventTypeFilters {
			if filter == event.Type() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check log level
	eventLevel := m.getEventLevel(event)
	return m.shouldLogLevel(eventLevel, m.config.LogLevel)
}

// getEventLevel determines the log level for an event.
func (m *EventLoggerModule) getEventLevel(event cloudevents.Event) string {
	// Map event types to log levels
	switch event.Type() {
	case modular.EventTypeApplicationFailed, modular.EventTypeModuleFailed:
		return "ERROR"
	case modular.EventTypeConfigValidated, modular.EventTypeConfigLoaded:
		return "DEBUG"
	default:
		return "INFO"
	}
}

// shouldLogLevel checks if a log level should be included based on minimum level.
func (m *EventLoggerModule) shouldLogLevel(eventLevel, minLevel string) bool {
	levels := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	eventLevelNum, ok1 := levels[eventLevel]
	minLevelNum, ok2 := levels[minLevel]

	if !ok1 || !ok2 {
		return true // Default to logging if levels are invalid
	}

	return eventLevelNum >= minLevelNum
}

// flushOutputs flushes all output targets.
func (m *EventLoggerModule) flushOutputs() {
	for _, output := range m.outputs {
		if err := output.Flush(); err != nil {
			m.logger.Error("Failed to flush output target", "error", err)
		}
	}
}

// LogEntry represents a log entry for an event.
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
