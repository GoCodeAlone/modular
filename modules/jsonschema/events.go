package jsonschema

// Event type constants for jsonschema module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Schema compilation events
	EventTypeSchemaCompiled = "com.modular.jsonschema.schema.compiled"
	EventTypeSchemaError    = "com.modular.jsonschema.schema.error"

	// Validation events
	EventTypeValidationSuccess = "com.modular.jsonschema.validation.success"
	EventTypeValidationFailed  = "com.modular.jsonschema.validation.failed"

	// Validation method events
	EventTypeValidateBytes     = "com.modular.jsonschema.validate.bytes"
	EventTypeValidateReader    = "com.modular.jsonschema.validate.reader"
	EventTypeValidateInterface = "com.modular.jsonschema.validate.interface"
)
