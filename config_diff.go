package modular

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ConfigDiff represents the differences between two configuration states.
// It tracks what fields have been added, changed, or removed, along with
// metadata about when the diff was generated and how to identify it.
//
// This type is used by the dynamic reload system to inform modules
// about exactly what has changed in their configuration, allowing them
// to make targeted updates rather than full reinitialization.
type ConfigDiff struct {
	// Changed maps field paths to their change information.
	// The key is the field path (e.g., "database.host", "api.timeout")
	// and the value contains the old and new values.
	Changed map[string]FieldChange

	// Added maps field paths to their new values.
	// These are fields that didn't exist in the previous configuration
	// but are present in the new configuration.
	Added map[string]interface{}

	// Removed maps field paths to their previous values.
	// These are fields that existed in the previous configuration
	// but are not present in the new configuration.
	Removed map[string]interface{}

	// Timestamp indicates when this diff was generated
	Timestamp time.Time

	// DiffID is a unique identifier for this configuration diff,
	// useful for tracking and correlation in logs and audit trails
	DiffID string
}

// ChangeType represents the type of change that occurred to a configuration field
type ChangeType string

const (
	// ChangeTypeAdded indicates a field was added to the configuration
	ChangeTypeAdded ChangeType = "added"
	
	// ChangeTypeModified indicates a field value was changed
	ChangeTypeModified ChangeType = "modified"
	
	// ChangeTypeRemoved indicates a field was removed from the configuration  
	ChangeTypeRemoved ChangeType = "removed"
)

// String returns the string representation of the change type
func (c ChangeType) String() string {
	return string(c)
}

// ValidationResult represents the result of validating a configuration change
type ValidationResult struct {
	// IsValid indicates whether the configuration change is valid
	IsValid bool
	
	// Message provides details about the validation result
	Message string
	
	// Warnings contains any validation warnings (non-fatal issues)
	Warnings []string
}

// ConfigChange represents a change in a specific configuration field.
// This is the structure used by the dynamic reload system to inform modules
// about configuration changes, following the design brief specifications.
type ConfigChange struct {
	// Section is the configuration section name (e.g., "database", "cache")
	Section string

	// FieldPath is the full dotted path to this field in the configuration
	// (e.g., "database.connection.host", "logging.level")
	FieldPath string

	// OldValue is the previous value of the field
	OldValue any

	// NewValue is the new value of the field
	NewValue any

	// Source is the feeder/source identifier that provided this change
	// (e.g., "env", "file:/config/app.yaml", "programmatic")
	Source string
}

// FieldChange represents a change in a specific configuration field.
// It captures both the previous and new values, along with metadata
// about the field and whether it contains sensitive information.
// 
// Deprecated: Use ConfigChange instead for new reload implementations.
// This type is maintained for backward compatibility.
type FieldChange struct {
	// OldValue is the previous value of the field
	OldValue interface{}

	// NewValue is the new value of the field
	NewValue interface{}

	// FieldPath is the full dotted path to this field in the configuration
	// (e.g., "database.connection.host", "logging.level")
	FieldPath string

	// ChangeType indicates what kind of change this represents
	ChangeType ChangeType

	// IsSensitive indicates whether this field contains sensitive information
	// that should be redacted from logs or audit trails
	IsSensitive bool
	
	// ValidationResult contains the result of validating this field change
	ValidationResult *ValidationResult
}

// ConfigFieldChange is an alias for FieldChange to maintain compatibility
// with existing test code while using the more descriptive FieldChange name
type ConfigFieldChange = FieldChange

// HasChanges returns true if the diff contains any changes
func (d *ConfigDiff) HasChanges() bool {
	return len(d.Changed) > 0 || len(d.Added) > 0 || len(d.Removed) > 0
}

// IsEmpty returns true if the diff contains no changes
func (d *ConfigDiff) IsEmpty() bool {
	return !d.HasChanges()
}

// GetChangedFields returns a slice of field paths that have changed values
func (d *ConfigDiff) GetChangedFields() []string {
	fields := make([]string, 0, len(d.Changed))
	for field := range d.Changed {
		fields = append(fields, field)
	}
	return fields
}

// GetAddedFields returns a slice of field paths that have been added
func (d *ConfigDiff) GetAddedFields() []string {
	fields := make([]string, 0, len(d.Added))
	for field := range d.Added {
		fields = append(fields, field)
	}
	return fields
}

// GetRemovedFields returns a slice of field paths that have been removed
func (d *ConfigDiff) GetRemovedFields() []string {
	fields := make([]string, 0, len(d.Removed))
	for field := range d.Removed {
		fields = append(fields, field)
	}
	return fields
}

// GetAllAffectedFields returns a slice of all field paths that are affected by this diff
func (d *ConfigDiff) GetAllAffectedFields() []string {
	allFields := make([]string, 0, len(d.Changed)+len(d.Added)+len(d.Removed))
	allFields = append(allFields, d.GetChangedFields()...)
	allFields = append(allFields, d.GetAddedFields()...)
	allFields = append(allFields, d.GetRemovedFields()...)
	return allFields
}

// RedactSensitiveFields returns a copy of the diff with sensitive field values redacted
func (d *ConfigDiff) RedactSensitiveFields() *ConfigDiff {
	redacted := &ConfigDiff{
		Changed:   make(map[string]FieldChange),
		Added:     make(map[string]interface{}),
		Removed:   make(map[string]interface{}),
		Timestamp: d.Timestamp,
		DiffID:    d.DiffID,
	}

	// Redact changed fields
	for path, change := range d.Changed {
		if change.IsSensitive {
			change.OldValue = "[REDACTED]"
			change.NewValue = "[REDACTED]"
		}
		redacted.Changed[path] = change
	}

	// Redact added fields - we need a way to know if they're sensitive
	// For now, copy as-is since we don't have metadata about sensitivity
	for path, value := range d.Added {
		redacted.Added[path] = value
	}

	// Redact removed fields - same issue as added
	for path, value := range d.Removed {
		redacted.Removed[path] = value
	}

	return redacted
}

// ChangeSummary provides a high-level summary of the configuration changes
type ChangeSummary struct {
	// TotalChanges is the total number of changes (added + modified + removed)
	TotalChanges int
	
	// AddedCount is the number of fields that were added
	AddedCount int
	
	// ModifiedCount is the number of fields that were modified
	ModifiedCount int
	
	// RemovedCount is the number of fields that were removed
	RemovedCount int
	
	// SensitiveChanges is the number of sensitive fields that were changed
	SensitiveChanges int
}

// ChangeSummary returns a summary of all changes in this diff
func (d *ConfigDiff) ChangeSummary() ChangeSummary {
	summary := ChangeSummary{
		AddedCount:    len(d.Added),
		ModifiedCount: len(d.Changed),
		RemovedCount:  len(d.Removed),
	}
	
	summary.TotalChanges = summary.AddedCount + summary.ModifiedCount + summary.RemovedCount
	
	// Count sensitive changes
	for _, change := range d.Changed {
		if change.IsSensitive {
			summary.SensitiveChanges++
		}
	}
	
	return summary
}

// FilterByPrefix returns a new ConfigDiff containing only changes to fields with the given prefix
func (d *ConfigDiff) FilterByPrefix(prefix string) *ConfigDiff {
	filtered := &ConfigDiff{
		Changed:   make(map[string]FieldChange),
		Added:     make(map[string]interface{}),
		Removed:   make(map[string]interface{}),
		Timestamp: d.Timestamp,
		DiffID:    d.DiffID + "-filtered",
	}
	
	// Filter changed fields
	for path, change := range d.Changed {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			filtered.Changed[path] = change
		}
	}
	
	// Filter added fields
	for path, value := range d.Added {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			filtered.Added[path] = value
		}
	}
	
	// Filter removed fields
	for path, value := range d.Removed {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			filtered.Removed[path] = value
		}
	}
	
	return filtered
}

// ConfigDiffOptions provides options for generating configuration diffs
type ConfigDiffOptions struct {
	// IgnoreFields is a list of field paths to ignore when generating the diff
	IgnoreFields []string
	
	// SensitiveFields is a list of field paths that should be marked as sensitive
	SensitiveFields []string
	
	// ValidateChanges indicates whether to validate changes during diff generation
	ValidateChanges bool
	
	// IncludeValidation indicates whether to include validation results in the diff
	IncludeValidation bool
	
	// MaxDepth limits how deep to recurse into nested structures
	MaxDepth int
}

// GenerateConfigDiff generates a diff between two configuration objects
func GenerateConfigDiff(oldConfig, newConfig interface{}) (*ConfigDiff, error) {
	return GenerateConfigDiffWithOptions(oldConfig, newConfig, ConfigDiffOptions{})
}

// GenerateConfigDiffWithOptions generates a diff with the specified options
func GenerateConfigDiffWithOptions(oldConfig, newConfig interface{}, options ConfigDiffOptions) (*ConfigDiff, error) {
	diff := &ConfigDiff{
		Changed:   make(map[string]FieldChange),
		Added:     make(map[string]interface{}),
		Removed:   make(map[string]interface{}),
		Timestamp: time.Now(),
		DiffID:    generateDiffID(),
	}
	
	// Convert configs to maps for easier comparison
	oldMap, err := configToMap(oldConfig, "")
	if err != nil {
		return nil, fmt.Errorf("failed to convert old config: %w", err)
	}
	
	newMap, err := configToMap(newConfig, "")
	if err != nil {
		return nil, fmt.Errorf("failed to convert new config: %w", err)
	}
	
	// Check for ignored fields
	ignoredFields := make(map[string]bool)
	for _, field := range options.IgnoreFields {
		ignoredFields[field] = true
	}
	
	// Check for sensitive fields
	sensitiveFields := make(map[string]bool)
	for _, field := range options.SensitiveFields {
		sensitiveFields[field] = true
	}
	
	// Find changed and removed fields
	for path, oldValue := range oldMap {
		if ignoredFields[path] {
			continue
		}
		
		if newValue, exists := newMap[path]; exists {
			// Field exists in both - check if changed
			if !compareValues(oldValue, newValue) {
				diff.Changed[path] = FieldChange{
					OldValue:    oldValue,
					NewValue:    newValue,
					FieldPath:   path,
					ChangeType:  ChangeTypeModified,
					IsSensitive: sensitiveFields[path],
				}
			}
		} else {
			// Field was removed
			diff.Removed[path] = oldValue
		}
	}
	
	// Find added fields
	for path, newValue := range newMap {
		if ignoredFields[path] {
			continue
		}
		
		if _, exists := oldMap[path]; !exists {
			// Field was added
			diff.Added[path] = newValue
		}
	}
	
	return diff, nil
}

// generateDiffID creates a unique identifier for a config diff
func generateDiffID() string {
	return time.Now().Format("20060102-150405.000000")
}

// configToMap converts a configuration object to a flattened map with dotted keys
func configToMap(config interface{}, prefix string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	if config == nil {
		return result, nil
	}
	
	value := reflect.ValueOf(config)
	
	// Handle pointers
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return result, nil
		}
		value = value.Elem()
	}
	
	switch value.Kind() {
	case reflect.Map:
		return mapToFlattened(config, prefix), nil
	case reflect.Struct:
		return structToFlattened(value, prefix), nil
	default:
		// For primitive values, use the prefix as the key
		if prefix != "" {
			result[prefix] = config
		}
		return result, nil
	}
}

// mapToFlattened converts a map to a flattened map with dotted keys
func mapToFlattened(config interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	
	value := reflect.ValueOf(config)
	if value.Kind() != reflect.Map {
		return result
	}
	
	for _, key := range value.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		fullKey := keyStr
		if prefix != "" {
			fullKey = prefix + "." + keyStr
		}
		
		mapValue := value.MapIndex(key).Interface()
		
		// Recursively flatten nested maps and structs
		if subMap, err := configToMap(mapValue, fullKey); err == nil {
			for subKey, subValue := range subMap {
				result[subKey] = subValue
			}
		} else {
			result[fullKey] = mapValue
		}
	}
	
	return result
}

// structToFlattened converts a struct to a flattened map with dotted keys
func structToFlattened(value reflect.Value, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	
	if value.Kind() != reflect.Struct {
		return result
	}
	
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)
		
		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}
		
		fieldName := strings.ToLower(fieldType.Name)
		fullKey := fieldName
		if prefix != "" {
			fullKey = prefix + "." + fieldName
		}
		
		fieldValue := field.Interface()
		
		// Recursively flatten nested structures
		if subMap, err := configToMap(fieldValue, fullKey); err == nil {
			for subKey, subValue := range subMap {
				result[subKey] = subValue
			}
		} else {
			result[fullKey] = fieldValue
		}
	}
	
	return result
}

// compareValues compares two values for equality
func compareValues(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// ReloadTrigger represents what triggered a configuration reload
type ReloadTrigger int

// Reload trigger constants
const (
	// ReloadTriggerManual indicates the reload was triggered manually
	ReloadTriggerManual ReloadTrigger = iota
	
	// ReloadTriggerFileChange indicates the reload was triggered by file changes
	ReloadTriggerFileChange
	
	// ReloadTriggerAPIRequest indicates the reload was triggered by API request
	ReloadTriggerAPIRequest
	
	// ReloadTriggerScheduled indicates the reload was triggered by schedule
	ReloadTriggerScheduled
)

// String returns the string representation of the reload trigger
func (r ReloadTrigger) String() string {
	switch r {
	case ReloadTriggerManual:
		return "manual"
	case ReloadTriggerFileChange:
		return "file_change"
	case ReloadTriggerAPIRequest:
		return "api_request"
	case ReloadTriggerScheduled:
		return "scheduled"
	default:
		return "unknown"
	}
}

// ParseReloadTrigger parses a string into a ReloadTrigger
func ParseReloadTrigger(s string) (ReloadTrigger, error) {
	switch s {
	case "manual":
		return ReloadTriggerManual, nil
	case "file_change":
		return ReloadTriggerFileChange, nil
	case "api_request":
		return ReloadTriggerAPIRequest, nil
	case "scheduled":
		return ReloadTriggerScheduled, nil
	default:
		return 0, fmt.Errorf("invalid reload trigger: %s", s)
	}
}

// Reload event types

// ConfigReloadStartedEvent represents an event emitted when a config reload starts
type ConfigReloadStartedEvent struct {
	// ReloadID is a unique identifier for this reload operation
	ReloadID string
	
	// Timestamp indicates when the reload started
	Timestamp time.Time
	
	// TriggerType indicates what triggered this reload
	TriggerType ReloadTrigger
	
	// ConfigDiff contains the configuration changes that triggered this reload
	ConfigDiff *ConfigDiff
}

// EventType returns the standardized event type for reload started events
func (e *ConfigReloadStartedEvent) EventType() string {
	return "config.reload.started"
}

// EventSource returns the standardized event source for reload started events  
func (e *ConfigReloadStartedEvent) EventSource() string {
	return "modular.core"
}

// GetEventType returns the type identifier for this event (implements ObserverEvent)
func (e *ConfigReloadStartedEvent) GetEventType() string {
	return e.EventType()
}

// GetEventSource returns the source that generated this event (implements ObserverEvent)
func (e *ConfigReloadStartedEvent) GetEventSource() string {
	return e.EventSource()
}

// GetTimestamp returns when this event occurred (implements ObserverEvent)
func (e *ConfigReloadStartedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// StructuredFields returns the structured field data for this event
func (e *ConfigReloadStartedEvent) StructuredFields() map[string]interface{} {
	fields := map[string]interface{}{
		"module":       "core",
		"phase":        "reload",
		"event":        "started",
		"reload_id":    e.ReloadID,
		"trigger_type": e.TriggerType.String(),
	}
	
	if e.ConfigDiff != nil {
		summary := e.ConfigDiff.ChangeSummary()
		fields["changes_count"] = summary.TotalChanges
		fields["added_count"] = summary.AddedCount
		fields["modified_count"] = summary.ModifiedCount
		fields["removed_count"] = summary.RemovedCount
	}
	
	return fields
}

// ConfigReloadCompletedEvent represents an event emitted when a config reload completes
type ConfigReloadCompletedEvent struct {
	// ReloadID is a unique identifier for this reload operation
	ReloadID string
	
	// Timestamp indicates when the reload completed
	Timestamp time.Time
	
	// Success indicates whether the reload was successful
	Success bool
	
	// Duration indicates how long the reload took
	Duration time.Duration
	
	// AffectedModules lists the modules that were affected by this reload
	AffectedModules []string
	
	// Error contains error details if Success is false
	Error string
	
	// ChangesApplied contains the number of configuration changes that were applied
	ChangesApplied int
}

// EventType returns the standardized event type for reload completed events
func (e *ConfigReloadCompletedEvent) EventType() string {
	return "config.reload.completed"
}

// EventSource returns the standardized event source for reload completed events  
func (e *ConfigReloadCompletedEvent) EventSource() string {
	return "modular.core"
}

// GetEventType returns the type identifier for this event (implements ObserverEvent)
func (e *ConfigReloadCompletedEvent) GetEventType() string {
	return e.EventType()
}

// GetEventSource returns the source that generated this event (implements ObserverEvent)
func (e *ConfigReloadCompletedEvent) GetEventSource() string {
	return e.EventSource()
}

// GetTimestamp returns when this event occurred (implements ObserverEvent)
func (e *ConfigReloadCompletedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// StructuredFields returns the structured field data for this event
func (e *ConfigReloadCompletedEvent) StructuredFields() map[string]interface{} {
	fields := map[string]interface{}{
		"module":           "core",
		"phase":            "reload",
		"event":            "completed",
		"reload_id":        e.ReloadID,
		"success":          e.Success,
		"duration_ms":      e.Duration.Milliseconds(),
		"changes_applied":  e.ChangesApplied,
	}
	
	if len(e.AffectedModules) > 0 {
		fields["affected_modules_count"] = len(e.AffectedModules)
		fields["affected_modules"] = e.AffectedModules
	}
	
	if !e.Success && e.Error != "" {
		fields["error"] = e.Error
	}
	
	return fields
}

// ConfigReloadFailedEvent represents an event emitted when a config reload fails
type ConfigReloadFailedEvent struct {
	// ReloadID is a unique identifier for this reload operation
	ReloadID string
	
	// Timestamp indicates when the reload failed
	Timestamp time.Time
	
	// Error contains the error that caused the failure
	Error string
	
	// FailedModule contains the name of the module that caused the failure (if applicable)
	FailedModule string
	
	// Duration indicates how long the reload attempt took before failing
	Duration time.Duration
}

// EventType returns the standardized event type for reload failed events
func (e *ConfigReloadFailedEvent) EventType() string {
	return "config.reload.failed"
}

// EventSource returns the standardized event source for reload failed events  
func (e *ConfigReloadFailedEvent) EventSource() string {
	return "modular.core"
}

// GetEventType returns the type identifier for this event (implements ObserverEvent)
func (e *ConfigReloadFailedEvent) GetEventType() string {
	return e.EventType()
}

// GetEventSource returns the source that generated this event (implements ObserverEvent)
func (e *ConfigReloadFailedEvent) GetEventSource() string {
	return e.EventSource()
}

// GetTimestamp returns when this event occurred (implements ObserverEvent)
func (e *ConfigReloadFailedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// ConfigReloadNoopEvent represents an event emitted when a config reload is a no-op
type ConfigReloadNoopEvent struct {
	// ReloadID is a unique identifier for this reload operation
	ReloadID string
	
	// Timestamp indicates when the no-op was determined
	Timestamp time.Time
	
	// Reason indicates why this was a no-op (e.g., "no changes detected")
	Reason string
}

// EventType returns the standardized event type for reload noop events
func (e *ConfigReloadNoopEvent) EventType() string {
	return "config.reload.noop"
}

// EventSource returns the standardized event source for reload noop events  
func (e *ConfigReloadNoopEvent) EventSource() string {
	return "modular.core"
}

// GetEventType returns the type identifier for this event (implements ObserverEvent)
func (e *ConfigReloadNoopEvent) GetEventType() string {
	return e.EventType()
}

// GetEventSource returns the source that generated this event (implements ObserverEvent)
func (e *ConfigReloadNoopEvent) GetEventSource() string {
	return e.EventSource()
}

// GetTimestamp returns when this event occurred (implements ObserverEvent)
func (e *ConfigReloadNoopEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// FilterEventsByReloadID filters a slice of observer events to include only reload events with the specified reload ID
func FilterEventsByReloadID(events []ObserverEvent, reloadID string) []ObserverEvent {
	var filtered []ObserverEvent
	
	for _, event := range events {
		switch reloadEvent := event.(type) {
		case *ConfigReloadStartedEvent:
			if reloadEvent.ReloadID == reloadID {
				filtered = append(filtered, event)
			}
		case *ConfigReloadCompletedEvent:
			if reloadEvent.ReloadID == reloadID {
				filtered = append(filtered, event)
			}
		case *ConfigReloadFailedEvent:
			if reloadEvent.ReloadID == reloadID {
				filtered = append(filtered, event)
			}
		case *ConfigReloadNoopEvent:
			if reloadEvent.ReloadID == reloadID {
				filtered = append(filtered, event)
			}
		}
	}
	
	return filtered
}