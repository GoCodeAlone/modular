package modular

import (
	"context"
	"testing"
)

func TestFunctionalObserver_New(t *testing.T) {
	called := false
	fo := NewFunctionalObserver("id1", func(ctx context.Context, e CloudEvent) error { called = true; return nil })
	if fo.ObserverID() != "id1" {
		t.Fatalf("id mismatch")
	}
	_ = fo.OnEvent(context.Background(), NewCloudEvent("t", "s", nil, nil))
	if !called {
		t.Fatalf("handler not called")
	}
}

func TestEventValidationObserver_New(t *testing.T) {
	expected := []string{"a", "b"}
	evo := NewEventValidationObserver("vid", expected)
	_ = evo.OnEvent(context.Background(), NewCloudEvent("a", "s", nil, nil))
	_ = evo.OnEvent(context.Background(), NewCloudEvent("c", "s", nil, nil))
	missing := evo.GetMissingEvents()
	if len(missing) != 1 || missing[0] != "b" {
		t.Fatalf("expected missing b, got %v", missing)
	}
	unexpected := evo.GetUnexpectedEvents()
	foundC := false
	for _, u := range unexpected {
		if u == "c" {
			foundC = true
		}
	}
	if !foundC {
		t.Fatalf("expected unexpected c event")
	}
	if len(evo.GetAllEvents()) != 2 {
		t.Fatalf("expected 2 events captured")
	}
	evo.Reset()
	if len(evo.GetAllEvents()) != 0 {
		t.Fatalf("expected reset to clear events")
	}
}
