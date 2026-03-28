package httpserver

// Event type constants for httpserver module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Server lifecycle events
	EventTypeServerStarted = "com.modular.httpserver.server.started"
	EventTypeServerStopped = "com.modular.httpserver.server.stopped"

	// Request handling events
	EventTypeRequestReceived = "com.modular.httpserver.request.received"
	EventTypeRequestHandled  = "com.modular.httpserver.request.handled"

	// TLS events
	EventTypeTLSEnabled    = "com.modular.httpserver.tls.enabled"
	EventTypeTLSConfigured = "com.modular.httpserver.tls.configured"

	// Configuration events
	EventTypeConfigLoaded = "com.modular.httpserver.config.loaded"
)
