package modular

import (
	"testing"
	"time"
)

// sample config structs for diffing
type cfgA struct {
	Host   string
	Port   int
	Nested struct {
		Enabled bool
	}
}

func TestGenerateConfigDiffBasic(t *testing.T) {
	oldC := &cfgA{Host: "localhost", Port: 8080}
	oldC.Nested.Enabled = true
	newC := &cfgA{Host: "example.com", Port: 9090}
	newC.Nested.Enabled = true // unchanged nested field

	diff, err := GenerateConfigDiff(oldC, newC)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}
	if diff.IsEmpty() {
		t.Fatalf("expected changes")
	}
	// Expect modified host, port; no nested change
	changed := diff.GetChangedFields()
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %d: %v", len(changed), changed)
	}
	summary := diff.ChangeSummary()
	if summary.ModifiedCount != 2 || summary.TotalChanges != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestGenerateConfigDiffOptionsIgnoreAndSensitive(t *testing.T) {
	oldM := map[string]any{"a": 1, "b": 2, "secret": "old"}
	newM := map[string]any{"a": 1, "b": 3, "secret": "new", "c": 4}
	diff, err := GenerateConfigDiffWithOptions(oldM, newM, ConfigDiffOptions{
		IgnoreFields:    []string{"a"},
		SensitiveFields: []string{"secret"},
	})
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}
	// a ignored; b modified; secret modified (sensitive); c added
	if _, ok := diff.Changed["a"]; ok {
		t.Fatalf("field a should be ignored")
	}
	if _, ok := diff.Changed["b"]; !ok {
		t.Fatalf("expected change for b")
	}
	if ch, ok := diff.Changed["secret"]; !ok || !ch.IsSensitive {
		t.Fatalf("expected sensitive secret change")
	}
	if _, ok := diff.Added["c"]; !ok {
		t.Fatalf("expected added c")
	}
	redacted := diff.RedactSensitiveFields()
	if redacted.Changed["secret"].OldValue != "[REDACTED]" {
		t.Fatalf("secret not redacted")
	}
}

func TestConfigDiffFilterByPrefix(t *testing.T) {
	base := &ConfigDiff{Changed: map[string]FieldChange{
		"db.host":     {FieldPath: "db.host", OldValue: "a", NewValue: "b"},
		"db.port":     {FieldPath: "db.port", OldValue: 1, NewValue: 2},
		"api.timeout": {FieldPath: "api.timeout", OldValue: 10, NewValue: 20},
	}, Added: map[string]any{"db.user": "u"}, Removed: map[string]any{"db.pass": "p"}, Timestamp: time.Now(), DiffID: "X"}
	filtered := base.FilterByPrefix("db.")
	if len(filtered.Changed) != 2 || len(filtered.Added) != 1 || len(filtered.Removed) != 1 {
		t.Fatalf("unexpected filtered counts: %+v", filtered)
	}
	if filtered.DiffID == base.DiffID {
		t.Fatalf("expected filtered diff id change")
	}
}

func TestConfigReloadEventsStructuredFields(t *testing.T) {
	// Build a diff for started event summary embedding
	d := &ConfigDiff{Changed: map[string]FieldChange{"x": {FieldPath: "x", OldValue: 1, NewValue: 2}}, Added: map[string]any{"y": 3}, Removed: map[string]any{}, Timestamp: time.Now(), DiffID: "id1"}
	start := &ConfigReloadStartedEvent{ReloadID: "rid", Timestamp: time.Now(), TriggerType: ReloadTriggerManual, ConfigDiff: d}
	sf := start.StructuredFields()
	if sf["modified_count"] != 1 || sf["added_count"] != 1 || sf["changes_count"].(int) != 2 {
		t.Fatalf("unexpected start structured fields: %+v", sf)
	}
	if start.GetEventType() == "" || start.GetEventSource() == "" || start.GetTimestamp().IsZero() {
		t.Fatalf("start event getters invalid")
	}

	complete := &ConfigReloadCompletedEvent{ReloadID: "rid", Timestamp: time.Now(), Success: true, Duration: 5 * time.Millisecond, AffectedModules: []string{"m1", "m2"}, ChangesApplied: 2}
	csf := complete.StructuredFields()
	if csf["affected_modules_count"] != 2 || csf["changes_applied"] != 2 {
		t.Fatalf("unexpected completed fields: %+v", csf)
	}
	if complete.GetEventType() == "" || complete.GetEventSource() == "" || complete.GetTimestamp().IsZero() {
		t.Fatalf("completed getters invalid")
	}

	failed := &ConfigReloadFailedEvent{ReloadID: "rid", Timestamp: time.Now(), Error: "boom", FailedModule: "m1", Duration: 3 * time.Millisecond}
	if failed.GetEventType() == "" || failed.GetEventSource() == "" || failed.GetTimestamp().IsZero() {
		t.Fatalf("failed getters invalid")
	}

	noop := &ConfigReloadNoopEvent{ReloadID: "rid", Timestamp: time.Now(), Reason: "no changes"}
	if noop.GetEventType() == "" || noop.GetEventSource() == "" || noop.GetTimestamp().IsZero() {
		t.Fatalf("noop getters invalid")
	}

	// FilterEventsByReloadID should return exactly four events for matching ID
	events := []ObserverEvent{start, complete, failed, noop}
	filtered := FilterEventsByReloadID(events, "rid")
	if len(filtered) != 4 {
		t.Fatalf("unexpected filtered length: %d", len(filtered))
	}
}
