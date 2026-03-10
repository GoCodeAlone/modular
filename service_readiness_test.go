package modular

import (
	"sync/atomic"
	"testing"
)

func TestOnServiceReady_AlreadyRegistered(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	registry.RegisterService("db", "postgres-conn")
	var called atomic.Bool
	registry.OnServiceReady("db", func(svc any) {
		called.Store(true)
		if svc != "postgres-conn" {
			t.Errorf("expected postgres-conn, got %v", svc)
		}
	})
	if !called.Load() {
		t.Error("callback should have been called immediately")
	}
}

func TestOnServiceReady_DeferredUntilRegistration(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	var called atomic.Bool
	var receivedSvc any
	registry.OnServiceReady("db", func(svc any) {
		called.Store(true)
		receivedSvc = svc
	})
	if called.Load() {
		t.Error("callback should not have been called yet")
	}
	registry.RegisterService("db", "postgres-conn")
	if !called.Load() {
		t.Error("callback should have been called after registration")
	}
	if receivedSvc != "postgres-conn" {
		t.Errorf("expected postgres-conn, got %v", receivedSvc)
	}
}

func TestOnServiceReady_MultipleCallbacks(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	var count atomic.Int32
	registry.OnServiceReady("cache", func(svc any) { count.Add(1) })
	registry.OnServiceReady("cache", func(svc any) { count.Add(1) })
	registry.RegisterService("cache", "redis")
	if count.Load() != 2 {
		t.Errorf("expected 2 callbacks, got %d", count.Load())
	}
}
