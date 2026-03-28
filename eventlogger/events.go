package eventlogger

// Event type constants for eventlogger module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Logger lifecycle events
	EventTypeLoggerStarted = "com.modular.eventlogger.started"
	EventTypeLoggerStopped = "com.modular.eventlogger.stopped"

	// Event processing events
	EventTypeEventReceived  = "com.modular.eventlogger.event.received"
	EventTypeEventProcessed = "com.modular.eventlogger.event.processed"
	EventTypeEventDropped   = "com.modular.eventlogger.event.dropped"

	// Buffer events
	EventTypeBufferFull = "com.modular.eventlogger.buffer.full"

	// Output events
	EventTypeOutputSuccess = "com.modular.eventlogger.output.success"
	EventTypeOutputError   = "com.modular.eventlogger.output.error"

	// Configuration events
	EventTypeConfigLoaded     = "com.modular.eventlogger.config.loaded"
	EventTypeOutputRegistered = "com.modular.eventlogger.output.registered"
)
