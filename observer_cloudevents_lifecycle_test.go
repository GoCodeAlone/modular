package modular

import (
	"encoding/json"
	"testing"
)

// TestNewModuleLifecycleEvent_Decode verifies we can round-trip the structured payload
// and that extension attributes are present for routing without decoding the data payload.
func TestNewModuleLifecycleEvent_Decode(t *testing.T) {
	evt := NewModuleLifecycleEvent("application", "module", "example", "v1.2.3", "started", map[string]interface{}{"key": "value"})

	if evt.Type() != EventTypeModuleStarted {
		t.Fatalf("unexpected type: %s", evt.Type())
	}
	if got := evt.Extensions()["payloadschema"]; got != ModuleLifecycleSchema {
		t.Fatalf("missing payloadschema extension: %v", got)
	}
	if got := evt.Extensions()["moduleaction"]; got != "started" {
		t.Fatalf("moduleaction extension mismatch: %v", got)
	}
	if got := evt.Extensions()["lifecyclesubject"]; got != "module" {
		t.Fatalf("lifecyclesubject mismatch: %v", got)
	}
	if got := evt.Extensions()["lifecyclename"]; got != "example" {
		t.Fatalf("lifecyclename mismatch: %v", got)
	}

	// Decode structured payload
	var pl ModuleLifecyclePayload
	if err := json.Unmarshal(evt.Data(), &pl); err != nil { // CloudEvents SDK stores raw bytes for JSON
		t.Fatalf("decode payload: %v", err)
	}
	if pl.Subject != "module" || pl.Name != "example" || pl.Action != "started" || pl.Version != "v1.2.3" {
		t.Fatalf("payload mismatch: %+v", pl)
	}
	if pl.Metadata["key"].(string) != "value" {
		t.Fatalf("metadata mismatch: %+v", pl.Metadata)
	}
}

// TestNewModuleLifecycleEvent_ApplicationSubject ensures application subject falls back to application type mapping.
func TestNewModuleLifecycleEvent_ApplicationSubject(t *testing.T) {
	evt := NewModuleLifecycleEvent("application", "application", "", "", "started", nil)
	if evt.Type() != EventTypeApplicationStarted {
		t.Fatalf("expected application started type, got %s", evt.Type())
	}
	if evt.Extensions()["lifecyclesubject"] != "application" {
		t.Fatalf("lifecyclesubject extension missing")
	}
}

// TestNewModuleLifecycleEvent_UnknownSubject ensures unknown subjects use generic lifecycle type.
func TestNewModuleLifecycleEvent_UnknownSubject(t *testing.T) {
	evt := NewModuleLifecycleEvent("application", "custom-subject", "", "", "custom", nil)
	if evt.Type() != "com.modular.lifecycle" { // generic fallback
		t.Fatalf("expected generic lifecycle type, got %s", evt.Type())
	}
	if evt.Extensions()["lifecyclesubject"] != "custom-subject" {
		t.Fatalf("lifecyclesubject extension mismatch")
	}
}
