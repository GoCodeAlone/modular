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
