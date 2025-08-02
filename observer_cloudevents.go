// Package modular provides CloudEvents integration for the Observer pattern.
// This file provides CloudEvents utility functions and validation for
// standardized event format and better interoperability.
package modular

import (
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
