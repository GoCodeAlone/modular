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
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
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
	name         string
	config       *EventLoggerConfig
	logger       modular.Logger
	outputs      []OutputTarget
	eventChan    chan cloudevents.Event
	stopChan     chan struct{}
	wg           sync.WaitGroup
	started      bool
	shuttingDown bool
	mutex        sync.RWMutex
	subject      modular.Subject
	// observerRegistered ensures we only register with the subject once
	observerRegistered bool
	// Event queueing for pre-start events - implements "queue until ready" approach
	// to handle events that arrive before Start() is called. This eliminates noise
	// from early lifecycle events while preserving all events for later processing.
	eventQueue   []cloudevents.Event
	queueMaxSize int
}

// setOutputsForTesting replaces the output targets. This is intended ONLY for
// test scenarios that need to inject faulty outputs after initialization. It
// acquires the module mutex to avoid data races with concurrent readers.
// NOTE: Mutating outputs at runtime is not supported in production usage.
//
//nolint:unused // Used in tests only
func (m *EventLoggerModule) setOutputsForTesting(outputs []OutputTarget) {
	m.mutex.Lock()
	m.outputs = outputs
	m.mutex.Unlock()
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
	// If a non-nil config provider is already registered (e.g., tests), don't override it
	if existing, err := app.GetConfigSection(m.Name()); err == nil && existing != nil {
		return nil
	}

	// Register the configuration with default values
	defaultConfig := &EventLoggerConfig{
		Enabled:              true,
		LogLevel:             "INFO",
		Format:               "structured",
		BufferSize:           100,
		FlushInterval:        5 * time.Second,
		IncludeMetadata:      true,
		IncludeStackTrace:    false,
		StartupSync:          false,
		ShutdownEmitStopped:  true,
		ShutdownDrainTimeout: 2 * time.Second,
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
	// Acquire write lock during initialization to avoid data races with OnEvent
	// which may run concurrently when early lifecycle events are emitted while
	// modules are still initializing. OnEvent reads fields like config, outputs,
	// eventQueue, queueMaxSize, started, and channels under the same mutex.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Retrieve the registered config section
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*EventLoggerConfig)
	m.logger = app.Logger()

	// Initialize output targets (still under lock for race safety)
	m.outputs = make([]OutputTarget, 0, len(m.config.OutputTargets))
	for i, targetConfig := range m.config.OutputTargets {
		output, err := NewOutputTarget(targetConfig, m.logger)
		if err != nil {
			return fmt.Errorf("failed to create output target %d: %w", i, err)
		}
		m.outputs = append(m.outputs, output)
	}

	// Initialize channels (protected by lock to prevent concurrent visibility of partially constructed state)
	m.eventChan = make(chan cloudevents.Event, m.config.BufferSize)
	m.stopChan = make(chan struct{})

	// Initialize event queue for pre-start events
	m.eventQueue = make([]cloudevents.Event, 0)
	m.queueMaxSize = 1000 // Reasonable limit to prevent memory issues

	if m.logger != nil {
		m.logger.Info("Event logger module initialized", "targets", len(m.outputs))
	}

	return nil
}

// Start starts the event logger processing.
func (m *EventLoggerModule) Start(ctx context.Context) error {
	m.mutex.Lock()

	if m.started {
		m.mutex.Unlock()
		return nil
	}

	// Guard against Start being called before Init (regression safety)
	if m.config == nil {
		if m.logger != nil {
			m.logger.Warn("Event logger Start called before Init; skipping")
		}
		m.mutex.Unlock()
		return nil
	}

	if !m.config.Enabled {
		if m.logger != nil {
			m.logger.Info("Event logger is disabled, skipping start")
		}
		m.mutex.Unlock()
		return nil
	}

	for _, output := range m.outputs { // start outputs
		if err := output.Start(ctx); err != nil {
			m.mutex.Unlock()
			return fmt.Errorf("failed to start output target: %w", err)
		}
	}

	m.wg.Add(1)
	go m.processEvents(ctx) // processEvents manages Done

	m.started = true
	if m.logger != nil {
		m.logger.Info("Event logger started")
	}

	// Process any queued events before normal operation
	queuedEvents := make([]cloudevents.Event, len(m.eventQueue))
	copy(queuedEvents, m.eventQueue)
	m.eventQueue = nil // Clear the queue

	// Capture data needed for emission outside the lock
	startupSync := m.config.StartupSync
	outputsLen := len(m.outputs)
	bufferLen := len(m.eventChan)
	outputConfigs := make([]OutputTargetConfig, len(m.config.OutputTargets))
	copy(outputConfigs, m.config.OutputTargets)

	// Release the lock before processing queued events to avoid deadlocks
	m.mutex.Unlock()

	// Process queued events synchronously to maintain order
	if len(queuedEvents) > 0 {
		if m.logger != nil {
			m.logger.Info("Processing queued events", "count", len(queuedEvents))
		}
		for _, event := range queuedEvents {
			m.logEvent(ctx, event)
		}
	}

	// Defer emission outside lock (no mutex needed since we released it)
	go m.emitStartupOperationalEvents(ctx, startupSync, outputsLen, bufferLen, outputConfigs)

	return nil
}

// emitStartupOperationalEvents performs the operational event emission without holding the Start mutex.
func (m *EventLoggerModule) emitStartupOperationalEvents(ctx context.Context, sync bool, outputsLen, bufferLen int, targetConfigs []OutputTargetConfig) {
	if m.logger == nil || m.config == nil || !m.started {
		/* nothing to emit or already stopped */
		return
	}
	emit := func(baseCtx context.Context) {
		m.emitOperationalEvent(baseCtx, EventTypeConfigLoaded, map[string]interface{}{
			"enabled":              m.config.Enabled,
			"buffer_size":          m.config.BufferSize,
			"output_targets_count": len(targetConfigs),
			"log_level":            m.config.LogLevel,
		})
		for i, tc := range targetConfigs {
			m.emitOperationalEvent(baseCtx, EventTypeOutputRegistered, map[string]interface{}{
				"output_index": i,
				"output_type":  tc.Type,
				"output_level": tc.Level,
			})
		}
		m.emitOperationalEvent(baseCtx, EventTypeLoggerStarted, map[string]interface{}{
			"output_count": outputsLen,
			"buffer_size":  bufferLen,
		})
	}
	if sync {
		emit(modular.WithSynchronousNotification(ctx))
	} else {
		emit(ctx)
	}
}

// Stop stops the event logger processing.
func (m *EventLoggerModule) Stop(ctx context.Context) error {
	m.mutex.Lock()
	if !m.started { // nothing to do
		m.mutex.Unlock()
		return nil
	}

	// Mark shutting down to suppress side-effects during drain
	m.shuttingDown = true

	// Capture config-driven behaviors then release lock so we can emit (emit acquires RLock)
	drainTimeout := m.config.ShutdownDrainTimeout
	emitStopped := m.config.ShutdownEmitStopped
	m.mutex.Unlock()

	// Emit the 'stopped' operational event BEFORE tearing down the processing goroutine
	if emitStopped {
		syncCtx := modular.WithSynchronousNotification(ctx)
		m.emitOperationalEvent(syncCtx, EventTypeLoggerStopped, map[string]interface{}{})
	}

	// Signal stop (idempotent safety)
	select {
	case <-m.stopChan:
	default:
		close(m.stopChan)
	}

	// Wait for processing with optional timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	if drainTimeout > 0 {
		select {
		case <-done:
		case <-time.After(drainTimeout):
			if m.logger != nil {
				m.logger.Warn("Event logger drain timeout reached; proceeding with shutdown")
			}
		}
	} else {
		<-done
	}

	// Stop outputs (independent of mutex)
	for _, output := range m.outputs {
		if err := output.Stop(ctx); err != nil && m.logger != nil {
			m.logger.Error("Failed to stop output target", "error", err)
		}
	}

	// Update state under lock again
	m.mutex.Lock()
	m.started = false
	if m.logger != nil {
		m.logger.Info("Event logger stopped")
	}
	// Clear shuttingDown flag
	m.shuttingDown = false
	m.mutex.Unlock()

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
	m.mutex.Lock()
	// Set subject reference for emitting operational events later
	m.subject = subject

	// Avoid duplicate registrations
	if m.observerRegistered {
		m.mutex.Unlock()
		if m.logger != nil {
			m.logger.Debug("RegisterObservers called - already registered, skipping")
		}
		return nil
	}

	// If config present but disabled, mark as handled and exit
	if m.config != nil && !m.config.Enabled {
		if m.logger != nil {
			m.logger.Info("Event logger is disabled, skipping observer registration")
		}
		m.observerRegistered = true
		m.mutex.Unlock()
		return nil
	}

	var err error
	if m.config != nil && len(m.config.EventTypeFilters) > 0 {
		err = subject.RegisterObserver(m, m.config.EventTypeFilters...)
		if err != nil {
			m.mutex.Unlock()
			return fmt.Errorf("failed to register event logger as observer: %w", err)
		}
		if m.logger != nil {
			m.logger.Info("Event logger registered as observer for filtered events", "filters", m.config.EventTypeFilters)
		}
	} else {
		err = subject.RegisterObserver(m)
		if err != nil {
			m.mutex.Unlock()
			return fmt.Errorf("failed to register event logger as observer: %w", err)
		}
		if m.logger != nil {
			m.logger.Info("Event logger registered as observer for all events")
		}
	}

	m.observerRegistered = true
	m.mutex.Unlock()
	return nil
}

// EmitEvent allows the module to emit its own operational events.
func (m *EventLoggerModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	m.mutex.RLock()
	subject := m.subject
	m.mutex.RUnlock()
	if subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// emitOperationalEvent emits an event about the eventlogger's own operations
func (m *EventLoggerModule) emitOperationalEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	m.mutex.RLock()
	if m.subject == nil {
		m.mutex.RUnlock()
		return // No subject available, skip event emission
	}
	m.mutex.RUnlock()

	event := modular.NewCloudEvent(eventType, "eventlogger-module", data, nil)

	// Check if synchronous notification is requested
	if modular.IsSynchronousNotification(ctx) {
		// Emit synchronously for reliable test capture
		if err := m.EmitEvent(ctx, event); err != nil {
			// Use the regular logger to avoid recursion
			m.logger.Debug("Failed to emit operational event", "error", err, "event_type", eventType)
		}
	} else {
		// Emit in background to avoid blocking operations and prevent infinite loops
		go func() {
			if err := m.EmitEvent(ctx, event); err != nil {
				// Use the regular logger to avoid recursion
				m.logger.Debug("Failed to emit operational event", "error", err, "event_type", eventType)
			}
		}()
	}
}

// isOwnEvent checks if an event is emitted by this eventlogger module to avoid infinite loops
func (m *EventLoggerModule) isOwnEvent(event cloudevents.Event) bool {
	// Treat events originating from this module OR having the eventlogger type prefix
	// as "own events" to avoid generating recursive operational events that amplify.
	if event.Source() == "eventlogger-module" {
		return true
	}
	// Defensive: if types are rewritten or forwarded and source lost, rely on type prefix.
	if strings.HasPrefix(event.Type(), "com.modular.eventlogger.") {
		return true
	}
	return false
}

// OnEvent implements the Observer interface to receive and log CloudEvents.
func (m *EventLoggerModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	// Check startup state and handle queueing with mutex protection
	var started bool
	var queueResult error
	var needsProcessing bool

	func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()

		started = m.started
		shuttingDown := m.shuttingDown

		if !started {
			if shuttingDown {
				// If we're shutting down, just drop the event silently
				queueResult = nil
				return
			}

			// If not initialized (eventQueue is nil), return error
			if m.eventQueue == nil {
				queueResult = ErrLoggerNotStarted
				return
			}

			// Queue the event until we're started (unless we're at queue limit)
			if len(m.eventQueue) < m.queueMaxSize {
				m.eventQueue = append(m.eventQueue, event)
				queueResult = nil
				return
			} else {
				// Queue is full - drop oldest event and add new one. We log both the incoming event type
				// and the dropped oldest event type for observability. This path intentionally avoids
				// emitting an operational CloudEvent because the logger itself is not yet started; emitting
				// here would risk recursive generation of events that also attempt to enqueue. Once started,
				// pressure signals are emitted via BufferFull/EventDropped events on the hot path with
				// safeguards to prevent amplification loops (see further below in non-started path logic).
				var droppedEventType string
				if len(m.eventQueue) > 0 {
					// Capture dropped event type for debugging visibility then shift slice
					droppedEventType = m.eventQueue[0].Type()
					copy(m.eventQueue, m.eventQueue[1:])
					m.eventQueue[len(m.eventQueue)-1] = event
				}
				if m.logger != nil {
					m.logger.Debug("Event queue full, dropped oldest event",
						"queue_size", m.queueMaxSize,
						"new_event", event.Type(),
						"dropped_event", droppedEventType)
				}
				queueResult = nil
				return
			}
		}

		needsProcessing = true
	}()

	// If we handled it during queueing phase, return early
	if !needsProcessing {
		return queueResult
	}

	// We're started - process normally
	// Cache ownership classification (hot path) to avoid repeated isOwnEvent calls for this event instance.
	isOwn := m.isOwnEvent(event)

	// Attempt non-blocking enqueue first. If it fails, channel is full and we must drop oldest.
	select {
	case m.eventChan <- event:
		// Enqueued successfully; record received (avoid loops for our own events)
		if !isOwn {
			m.emitOperationalEvent(ctx, EventTypeEventReceived, map[string]interface{}{
				"event_type":   event.Type(),
				"event_source": event.Source(),
			})
		}
		return nil
	default:
		// Full — drop oldest (non-blocking) then try again.
		// IMPORTANT: If the current event is an operational (own) event, we must avoid
		// emitting further BufferFull / EventDropped operational events, because those
		// themselves are operational events and would recursively trigger this path
		// while the channel remains saturated, leading to unbounded recursion and
		// eventual stack overflow (observed in TestEventLogger_SynchronousStartupConfigFlag).
		var dropped *cloudevents.Event
		select {
		case old := <-m.eventChan:
			dropped = &old
		default:
			// Nothing to drop (capacity might be 0); we'll treat as dropping the new event below if second send fails.
		}

		if !isOwn && dropped != nil {
			// Only emit pressure events if the triggering event is external.
			syncCtx := modular.WithSynchronousNotification(ctx)
			m.emitOperationalEvent(syncCtx, EventTypeBufferFull, map[string]interface{}{
				"buffer_size": cap(m.eventChan),
			})
			if !m.isOwnEvent(*dropped) { // avoid amplification chains from internal events
				m.emitOperationalEvent(syncCtx, EventTypeEventDropped, map[string]interface{}{
					"event_type":   dropped.Type(),
					"event_source": dropped.Source(),
					"reason":       "buffer_full_oldest_dropped",
				})
			}
		}

		// Retry enqueue of current event.
		select {
		case m.eventChan <- event:
			if !isOwn {
				m.emitOperationalEvent(ctx, EventTypeEventReceived, map[string]interface{}{
					"event_type":   event.Type(),
					"event_source": event.Source(),
				})
			}
			return nil
		default:
			// Still full (or capacity 0) — drop incoming event.
			if !isOwn { // only warn & emit for external events
				m.logger.Warn("Event buffer full, dropping incoming event", "eventType", event.Type())
				syncCtx := modular.WithSynchronousNotification(ctx)
				m.emitOperationalEvent(syncCtx, EventTypeBufferFull, map[string]interface{}{
					"buffer_size": cap(m.eventChan),
				})
				m.emitOperationalEvent(syncCtx, EventTypeEventDropped, map[string]interface{}{
					"event_type":   event.Type(),
					"event_source": event.Source(),
					"reason":       "buffer_full_incoming_dropped",
				})
			}
			return ErrEventBufferFull
		}
	}
}

// ObserverID returns the unique identifier for this observer.
func (m *EventLoggerModule) ObserverID() string {
	return ModuleName
}

// processEvents processes events from both event channels.
func (m *EventLoggerModule) processEvents(ctx context.Context) {
	defer m.wg.Done()

	flushTicker := time.NewTicker(m.config.FlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case event := <-m.eventChan:
			m.logEvent(ctx, event)

			// Emit event processed event (avoid emitting for our own events to prevent loops)
			if !m.isOwnEvent(event) {
				m.emitOperationalEvent(ctx, EventTypeEventProcessed, map[string]interface{}{
					"event_type":   event.Type(),
					"event_source": event.Source(),
				})
			}

		case <-flushTicker.C:
			m.flushOutputs()

		case <-m.stopChan:
			// Process remaining events
			for {
				select {
				case event := <-m.eventChan:
					m.logEvent(ctx, event)

					// Emit event processed event (avoid emitting for our own events to prevent loops)
					if !m.isOwnEvent(event) {
						m.emitOperationalEvent(ctx, EventTypeEventProcessed, map[string]interface{}{
							"event_type":   event.Type(),
							"event_source": event.Source(),
						})
					}
				default:
					m.flushOutputs()
					return
				}
			}
		}
	}
}

// logEvent logs a CloudEvent to all configured output targets.
func (m *EventLoggerModule) logEvent(ctx context.Context, event cloudevents.Event) {
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

	// Snapshot outputs under read lock to avoid races with test mutations.
	m.mutex.RLock()
	outputs := make([]OutputTarget, len(m.outputs))
	copy(outputs, m.outputs)
	m.mutex.RUnlock()

	// Send to all output targets
	successCount := 0
	errorCount := 0

	for _, output := range outputs {
		if err := output.WriteEvent(entry); err != nil {
			m.logger.Error("Failed to write event to output target", "error", err, "eventType", event.Type())
			errorCount++

			// Emit output error event (avoid emitting for our own events to prevent loops)
			if !m.isOwnEvent(event) {
				m.emitOperationalEvent(ctx, EventTypeOutputError, map[string]interface{}{
					"error":        err.Error(),
					"event_type":   event.Type(),
					"event_source": event.Source(),
				})
			}
		} else {
			successCount++
		}
	}

	// Emit output success event synchronously if at least one output succeeded (avoid emitting for our own events)
	if successCount > 0 && !m.isOwnEvent(event) {
		syncCtx := modular.WithSynchronousNotification(ctx)
		m.emitOperationalEvent(syncCtx, EventTypeOutputSuccess, map[string]interface{}{
			"success_count": successCount,
			"error_count":   errorCount,
			"event_type":    event.Type(),
			"event_source":  event.Source(),
		})
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
	m.mutex.RLock()
	outputs := make([]OutputTarget, len(m.outputs))
	copy(outputs, m.outputs)
	m.mutex.RUnlock()
	for _, output := range outputs {
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

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this eventlogger module can emit.
func (m *EventLoggerModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeLoggerStarted,
		EventTypeLoggerStopped,
		EventTypeEventReceived,
		EventTypeEventProcessed,
		EventTypeEventDropped,
		EventTypeBufferFull,
		EventTypeOutputSuccess,
		EventTypeOutputError,
		EventTypeConfigLoaded,
		EventTypeOutputRegistered,
	}
}
