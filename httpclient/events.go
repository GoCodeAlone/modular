package httpclient

// Event type constants for httpclient module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Client lifecycle events
	EventTypeClientCreated    = "com.modular.httpclient.client.created"
	EventTypeClientStarted    = "com.modular.httpclient.client.started"
	EventTypeClientConfigured = "com.modular.httpclient.client.configured"

	// Request modifier events
	EventTypeModifierSet     = "com.modular.httpclient.modifier.set"
	EventTypeModifierApplied = "com.modular.httpclient.modifier.applied"
	EventTypeModifierAdded   = "com.modular.httpclient.modifier.added"
	EventTypeModifierRemoved = "com.modular.httpclient.modifier.removed"

	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.httpclient.module.started"
	EventTypeModuleStopped = "com.modular.httpclient.module.stopped"

	// Configuration events
	EventTypeConfigLoaded   = "com.modular.httpclient.config.loaded"
	EventTypeTimeoutChanged = "com.modular.httpclient.timeout.changed"
)
