package chimux

// Event type constants for chimux module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Configuration events
	EventTypeConfigLoaded    = "com.modular.chimux.config.loaded"
	EventTypeConfigValidated = "com.modular.chimux.config.validated"

	// Router events
	EventTypeRouterCreated = "com.modular.chimux.router.created"
	EventTypeRouterStarted = "com.modular.chimux.router.started"
	EventTypeRouterStopped = "com.modular.chimux.router.stopped"

	// Route events
	EventTypeRouteRegistered = "com.modular.chimux.route.registered"
	EventTypeRouteRemoved    = "com.modular.chimux.route.removed"

	// Middleware events
	EventTypeMiddlewareAdded   = "com.modular.chimux.middleware.added"
	EventTypeMiddlewareRemoved = "com.modular.chimux.middleware.removed"

	// CORS events
	EventTypeCorsConfigured = "com.modular.chimux.cors.configured"
	EventTypeCorsEnabled    = "com.modular.chimux.cors.enabled"

	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.chimux.module.started"
	EventTypeModuleStopped = "com.modular.chimux.module.stopped"

	// Request processing events
	EventTypeRequestReceived  = "com.modular.chimux.request.received"
	EventTypeRequestProcessed = "com.modular.chimux.request.processed"
	EventTypeRequestFailed    = "com.modular.chimux.request.failed"
)
