// Package modular provides CloudEvents integration for the Observer pattern.
// This file provides CloudEvents utility functions and validation for
// standardized event format and better interoperability.
package modular

import (
	"errors"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
)

// CloudEvent is an alias for the CloudEvents Event type for convenience
type CloudEvent = cloudevents.Event

// NewCloudEvent creates a new CloudEvent with the specified parameters.
// This is a convenience function for creating properly formatted CloudEvents.
func NewCloudEvent(eventType, source string, data interface{}, metadata map[string]interface{}) cloudevents.Event {
	event := cloudevents.NewEvent()

	// Set required attributes
	event.SetID(generateEventID())
	event.SetSource(source)
	event.SetType(eventType)
	event.SetTime(time.Now())
	event.SetSpecVersion(cloudevents.VersionV1)

	// Set data if provided
	if data != nil {
		_ = event.SetData(cloudevents.ApplicationJSON, data)
	}

	// Set extensions for metadata
	for key, value := range metadata {
		event.SetExtension(key, value)
	}

	return event
}

// ModuleLifecycleSchema is the schema identifier for module lifecycle payloads.
const ModuleLifecycleSchema = "modular.module.lifecycle.v1"

// ModuleLifecyclePayload represents a structured lifecycle event for a module or the application.
// This provides a strongly-typed alternative to scattering lifecycle details across CloudEvent extensions.
// Additional routing-friendly metadata (like action) is still duplicated into a small extension for fast filtering.
type ModuleLifecyclePayload struct {
	// Subject indicates whether this is a module or application lifecycle event (e.g., "module", "application").
	Subject string `json:"subject"`
	// Name is the module/application name.
	Name string `json:"name"`
	// Action is the lifecycle action (e.g., start|stop|init|register|fail|initialize|initialized).
	Action string `json:"action"`
	// Version optionally records the module version if available.
	Version string `json:"version,omitempty"`
	// Timestamp is when the lifecycle action occurred (RFC3339 in JSON output).
	Timestamp time.Time `json:"timestamp"`
	// Additional arbitrary metadata (kept minimal; prefer evolving the struct if fields become first-class).
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewModuleLifecycleEvent builds a CloudEvent for a module/application lifecycle using the structured payload.
// It sets payload_schema and module_action extensions for lightweight routing without full payload decode.
func NewModuleLifecycleEvent(source, subject, name, version, action string, metadata map[string]interface{}) cloudevents.Event {
	payload := ModuleLifecyclePayload{
		Subject:   subject,
		Name:      name,
		Action:    action,
		Version:   version,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	evt := cloudevents.NewEvent()
	evt.SetID(generateEventID())
	evt.SetSource(source)
	// Keep specific event type naming for backward compatibility where possible (module/application generic fallback)
	switch subject {
	case "module":
		// Derive a conventional type if action matches known ones
		switch action {
		case "registered":
			evt.SetType(EventTypeModuleRegistered)
		case "initialized":
			evt.SetType(EventTypeModuleInitialized)
		case "started":
			evt.SetType(EventTypeModuleStarted)
		case "stopped":
			evt.SetType(EventTypeModuleStopped)
		case "failed":
			evt.SetType(EventTypeModuleFailed)
		default:
			evt.SetType("com.modular.module.lifecycle")
		}
	case "application":
		switch action {
		case "started":
			evt.SetType(EventTypeApplicationStarted)
		case "stopped":
			evt.SetType(EventTypeApplicationStopped)
		case "failed":
			evt.SetType(EventTypeApplicationFailed)
		default:
			evt.SetType("com.modular.application.lifecycle")
		}
	default:
		evt.SetType("com.modular.lifecycle")
	}
	evt.SetTime(payload.Timestamp)
	evt.SetSpecVersion(cloudevents.VersionV1)
	_ = evt.SetData(cloudevents.ApplicationJSON, payload)
	// CloudEvents 1.0 spec (section 3.1.1) restricts extension attribute names to **lower-case alphanumerics only**
	// (regex: [a-z0-9]{1,20}). Hyphens / underscores are NOT permitted in extension names. The reviewer suggested
	// using hyphens for readability; we intentionally retain plain concatenated names to remain strictly
	// compliant with the spec across all transports and SDKs. If readability / grouping is desired downstream,
	// mapping can be performed externally (e.g. transforming to labels / tags). These names are therefore
	// intentionally left without separators.
	evt.SetExtension("payloadschema", ModuleLifecycleSchema)
	evt.SetExtension("moduleaction", action)
	evt.SetExtension("lifecyclesubject", subject)
	evt.SetExtension("lifecyclename", name)
	return evt
}

// generateEventID generates a unique identifier for CloudEvents using UUIDv7.
// UUIDv7 includes timestamp information which provides time-ordered uniqueness.
func generateEventID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to v4 if v7 fails for any reason
		id = uuid.New()
	}
	return id.String()
}

// ValidateCloudEvent validates that a CloudEvent conforms to the specification.
// This provides validation beyond the basic CloudEvent SDK validation.
func ValidateCloudEvent(event cloudevents.Event) error {
	// Use the CloudEvent SDK's built-in validation
	if err := event.Validate(); err != nil {
		return fmt.Errorf("CloudEvent validation failed: %w", err)
	}

	// Additional validation could be added here for application-specific requirements
	return nil
}

// HandleEventEmissionError provides consistent error handling for event emission failures.
// This helper function standardizes how modules should handle the "no subject available" error
// and other emission failures to reduce noisy output during tests and in non-observable applications.
//
// It returns true if the error was handled (i.e., it was ErrNoSubjectForEventEmission or similar),
// false if the error should be handled by the caller.
//
// Example usage:
//
//	if err := module.EmitEvent(ctx, event); err != nil {
//		if !modular.HandleEventEmissionError(err, logger, "my-module", eventType) {
//			// Handle other types of errors here
//		}
//	}
func HandleEventEmissionError(err error, logger Logger, moduleName, eventType string) bool {
	// Handle the common "no subject available" error by silently ignoring it
	if errors.Is(err, ErrNoSubjectForEventEmission) {
		return true
	}

	// Also check for module-specific variants that have the same message
	if err.Error() == "no subject available for event emission" {
		return true
	}

	// Log other errors using structured logging if logger is available
	if logger != nil {
		logger.Debug("Failed to emit event", "module", moduleName, "eventType", eventType, "error", err)
		return true
	}

	// If no logger available, error wasn't the "no subject" error, let caller handle it
	return false
}
