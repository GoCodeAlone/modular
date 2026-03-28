package logmasker

// Event type constants for LogMasker module
// Following CloudEvents specification with reverse domain notation
const (
	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.logmasker.started"
	EventTypeModuleStopped = "com.modular.logmasker.stopped"

	// Configuration events
	EventTypeConfigLoaded    = "com.modular.logmasker.config.loaded"
	EventTypeConfigValidated = "com.modular.logmasker.config.validated"
	EventTypeRulesUpdated    = "com.modular.logmasker.rules.updated"

	// Masking operation events
	EventTypeMaskingApplied = "com.modular.logmasker.masking.applied"
	EventTypeMaskingSkipped = "com.modular.logmasker.masking.skipped"
	EventTypeFieldMasked    = "com.modular.logmasker.field.masked"
	EventTypePatternMatched = "com.modular.logmasker.pattern.matched"
	EventTypeMaskingError   = "com.modular.logmasker.masking.error"
)
