package modular

import (
	"testing"
	"time"
)

// sample nested config structs for diff flattening
type diffCfgA struct {
	Database struct {
		Host string
		Port int
	}
	Secret string
}
type diffCfgB struct {
	Database struct {
		Host string
		Port int
	}
	Secret  string
	Feature struct{ Enabled bool }
}

// testConfig needed by benchmark file and kept minimal here
type testConfig struct {
	DatabaseHost string `json:"database_host"`
	ServerPort   int    `json:"server_port"`
	CacheTTL     string `json:"cache_ttl"`
}

func TestConfigDiffBasicAccessorsAndRedaction(t *testing.T) {
	// prepare old/new
	var oldA diffCfgA
	oldA.Database.Host = "db.local"
	oldA.Database.Port = 5432
	oldA.Secret = "shh"
	var newB diffCfgB
	newB.Database.Host = "db.internal"
	newB.Database.Port = 5432
	newB.Secret = "shh-new"
	newB.Feature.Enabled = true

	diff, err := GenerateConfigDiffWithOptions(oldA, newB, ConfigDiffOptions{SensitiveFields: []string{"secret"}})
	if err != nil {
		t.Fatalf("diff err: %v", err)
	}

	if diff.IsEmpty() {
		t.Fatalf("expected changes")
	}
	changed := diff.GetChangedFields()
	added := diff.GetAddedFields()
	removed := diff.GetRemovedFields()
	all := diff.GetAllAffectedFields()
	if len(changed) == 0 || len(all) < len(changed) {
		t.Fatalf("expected changed included in all")
	}
	if len(added) == 0 {
		t.Fatalf("expected added field")
	}
	if len(removed) != 0 {
		t.Fatalf("no removed fields expected")
	}

	// mark one field as sensitive manually by editing diff.Changed (since sensitive only tracked there)
	for k, v := range diff.Changed {
		if k == "secret" {
			v.IsSensitive = true
			diff.Changed[k] = v
		}
	}
	red := diff.RedactSensitiveFields()
	if red.Changed["secret"].OldValue != "[REDACTED]" {
		t.Fatalf("expected redaction")
	}

	// Filter prefix
	dbOnly := diff.FilterByPrefix("database")
	for k := range dbOnly.Changed {
		if k[:8] != "database" {
			t.Fatalf("unexpected key %s", k)
		}
	}
	for k := range dbOnly.Added {
		if k[:8] != "database" {
			t.Fatalf("unexpected key %s", k)
		}
	}

	summary := diff.ChangeSummary()
	if summary.TotalChanges == 0 || summary.ModifiedCount == 0 {
		t.Fatalf("summary counts incorrect: %+v", summary)
	}
}

func TestConfigReloadEventsStructuredFieldsAndFilter(t *testing.T) {
	diff := &ConfigDiff{Added: map[string]interface{}{"a": 1}, Changed: map[string]FieldChange{"b": {OldValue: 1, NewValue: 2, FieldPath: "b", ChangeType: ChangeTypeModified}}, Removed: map[string]interface{}{}, Timestamp: time.Now(), DiffID: "x"}
	start := &ConfigReloadStartedEvent{ReloadID: "rid", Timestamp: time.Now(), TriggerType: ReloadTriggerAPIRequest, ConfigDiff: diff}
	fields := start.StructuredFields()
	if fields["changes_count"].(int) != 2 {
		t.Fatalf("expected 2 changes")
	}
	comp := &ConfigReloadCompletedEvent{ReloadID: "rid", Timestamp: time.Now(), Success: true, Duration: 5 * time.Millisecond, ChangesApplied: 2, AffectedModules: []string{"m1"}}
	fail := &ConfigReloadFailedEvent{ReloadID: "rid", Timestamp: time.Now(), Error: "boom"}
	noop := &ConfigReloadNoopEvent{ReloadID: "rid", Timestamp: time.Now(), Reason: "none"}
	events := []ObserverEvent{start, comp, fail, noop}
	filtered := FilterEventsByReloadID(events, "rid")
	if len(filtered) != 4 {
		t.Fatalf("expected 4 events got %d", len(filtered))
	}
}
