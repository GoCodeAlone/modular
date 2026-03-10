package modular

import (
	"fmt"
	"strings"
	"time"
)

// ChangeType represents the type of configuration change.
type ChangeType int

const (
	// ChangeAdded indicates a new configuration field was added.
	ChangeAdded ChangeType = iota
	// ChangeModified indicates an existing configuration field was modified.
	ChangeModified
	// ChangeRemoved indicates a configuration field was removed.
	ChangeRemoved
)

// String returns the string representation of a ChangeType.
func (ct ChangeType) String() string {
	switch ct {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

// ConfigChange represents a single configuration change detected during reload.
type ConfigChange struct {
	Section   string
	FieldPath string
	OldValue  string
	NewValue  string
	Source    string
}

// FieldChange represents a detailed field-level change with validation metadata.
type FieldChange struct {
	OldValue         any
	NewValue         any
	FieldPath        string
	ChangeType       ChangeType
	IsSensitive      bool
	ValidationResult error
}

// ConfigDiff represents the complete set of configuration changes between two states.
type ConfigDiff struct {
	Changed   map[string]FieldChange
	Added     map[string]FieldChange
	Removed   map[string]FieldChange
	Timestamp time.Time
	DiffID    string
}

// HasChanges reports whether the diff contains any changes.
func (d ConfigDiff) HasChanges() bool {
	return len(d.Changed) > 0 || len(d.Added) > 0 || len(d.Removed) > 0
}

// FilterByPrefix returns a new ConfigDiff containing only changes whose field paths
// start with the given prefix.
func (d ConfigDiff) FilterByPrefix(prefix string) ConfigDiff {
	filtered := ConfigDiff{
		Changed:   make(map[string]FieldChange),
		Added:     make(map[string]FieldChange),
		Removed:   make(map[string]FieldChange),
		Timestamp: d.Timestamp,
		DiffID:    d.DiffID,
	}
	for k, v := range d.Changed {
		if strings.HasPrefix(k, prefix) {
			filtered.Changed[k] = v
		}
	}
	for k, v := range d.Added {
		if strings.HasPrefix(k, prefix) {
			filtered.Added[k] = v
		}
	}
	for k, v := range d.Removed {
		if strings.HasPrefix(k, prefix) {
			filtered.Removed[k] = v
		}
	}
	return filtered
}

// RedactSensitiveFields returns a copy of the diff with sensitive field values replaced
// by a redaction placeholder.
func (d ConfigDiff) RedactSensitiveFields() ConfigDiff {
	redacted := ConfigDiff{
		Changed:   make(map[string]FieldChange, len(d.Changed)),
		Added:     make(map[string]FieldChange, len(d.Added)),
		Removed:   make(map[string]FieldChange, len(d.Removed)),
		Timestamp: d.Timestamp,
		DiffID:    d.DiffID,
	}
	redactMap := func(src map[string]FieldChange, dst map[string]FieldChange) {
		for k, v := range src {
			if v.IsSensitive {
				v.OldValue = "[REDACTED]"
				v.NewValue = "[REDACTED]"
			}
			dst[k] = v
		}
	}
	redactMap(d.Changed, redacted.Changed)
	redactMap(d.Added, redacted.Added)
	redactMap(d.Removed, redacted.Removed)
	return redacted
}

// ChangeSummary returns a human-readable summary of all changes in the diff.
func (d ConfigDiff) ChangeSummary() string {
	if !d.HasChanges() {
		return "no changes"
	}
	var parts []string
	if n := len(d.Added); n > 0 {
		parts = append(parts, fmt.Sprintf("%d added", n))
	}
	if n := len(d.Changed); n > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", n))
	}
	if n := len(d.Removed); n > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", n))
	}
	return strings.Join(parts, ", ")
}

// ReloadTrigger indicates what initiated a configuration reload.
type ReloadTrigger int

const (
	// ReloadManual indicates a reload triggered by an explicit API or CLI call.
	ReloadManual ReloadTrigger = iota
	// ReloadFileChange indicates a reload triggered by a file system change.
	ReloadFileChange
	// ReloadAPIRequest indicates a reload triggered by an API request.
	ReloadAPIRequest
	// ReloadScheduled indicates a reload triggered by a periodic schedule.
	ReloadScheduled
)

// String returns the string representation of a ReloadTrigger.
func (rt ReloadTrigger) String() string {
	switch rt {
	case ReloadManual:
		return "manual"
	case ReloadFileChange:
		return "file_change"
	case ReloadAPIRequest:
		return "api_request"
	case ReloadScheduled:
		return "scheduled"
	default:
		return "unknown"
	}
}
