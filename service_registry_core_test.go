package modular

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestEnhancedServiceRegistry_RegisterAndGet basic happy path.
func TestEnhancedServiceRegistry_RegisterAndGet(t *testing.T) {
	reg := NewEnhancedServiceRegistry()
	// Register without current module
	name, err := reg.RegisterService("logger", &struct{ Level string }{Level: "info"})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if name != "logger" {
		t.Fatalf("expected name logger, got %s", name)
	}

	svc, ok := reg.GetService("logger")
	if !ok || svc == nil {
		t.Fatalf("expected to retrieve service")
	}

	entry, ok := reg.GetServiceEntry("logger")
	if !ok || entry.OriginalName != "logger" || entry.ActualName != "logger" {
		t.Fatalf("unexpected entry %+v", entry)
	}
}

// TestEnhancedServiceRegistry_ConflictResolution ensures unique naming strategy path coverage.
func TestEnhancedServiceRegistry_ConflictResolution(t *testing.T) {
	reg := NewEnhancedServiceRegistry()
	// Simulate module context
	reg.SetCurrentModule(&testSimpleModule{name: "alpha"})
	_, _ = reg.RegisterService("cache", 1)
	// Second registration with same original name and module -> should remain original for first, second conflict triggers module name variant
	reg.SetCurrentModule(&testSimpleModule{name: "beta"})
	secondName, _ := reg.RegisterService("cache", 2)
	if secondName == "cache" {
		t.Fatalf("expected conflict rename, got same name")
	}
	// Force further conflicts to hit numeric suffix path
	reg.SetCurrentModule(&testSimpleModule{name: "alpha"})
	_, _ = reg.RegisterService("cache", 3)
	reg.SetCurrentModule(&testSimpleModule{name: "beta"})
	fourthName, _ := reg.RegisterService("cache", 4)
	if fourthName == "cache" {
		t.Fatalf("expected unique name for fourth registration")
	}
	if fourthName == secondName {
		t.Fatalf("expected different name for later conflict")
	}
	reg.ClearCurrentModule()
}

// TestEnhancedServiceRegistry_InterfaceQuery ensures GetServicesByInterface branch where service implements interface.
type demoIFace interface{ demo() }
type demoImpl struct{}

func (demoImpl) demo() {}

func TestEnhancedServiceRegistry_InterfaceQuery(t *testing.T) {
	reg := NewEnhancedServiceRegistry()
	_, _ = reg.RegisterService("impl", demoImpl{})
	matches := reg.GetServicesByInterface(reflect.TypeOf((*demoIFace)(nil)).Elem())
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

// TestScopedServiceRegistry_Scopes covers singleton, transient, scoped, default, and error path.
func TestScopedServiceRegistry_Scopes(t *testing.T) {
	reg := NewServiceRegistry()
	// Configure scopes
	_ = reg.ApplyOption(WithServiceScope("single", ServiceScopeSingleton))
	_ = reg.ApplyOption(WithServiceScope("trans", ServiceScopeTransient))
	_ = reg.ApplyOption(WithServiceScopeConfig("scoped", ServiceScopeConfig{Scope: ServiceScopeScoped, ScopeKey: "tenant"}))

	// Register factories
	reg.Register("single", func() *struct{ ID int } { return &struct{ ID int }{ID: 1} })
	reg.Register("trans", func() *struct{ ID int } { return &struct{ ID int }{ID: 1} })
	counter := 0
	reg.Register("scoped", func() *struct{ C int } { counter++; return &struct{ C int }{C: counter} })

	a1, _ := reg.Get("single")
	a2, _ := reg.Get("single")
	if a1 != a2 {
		t.Fatalf("singleton instances differ")
	}

	t1, _ := reg.Get("trans")
	t2, _ := reg.Get("trans")
	if t1 == t2 {
		t.Fatalf("transient instances same")
	}

	ctxA := WithScopeContext(context.Background(), "tenant", "tA")
	ctxB := WithScopeContext(context.Background(), "tenant", "tB")
	s1a, _ := reg.GetWithContext(ctxA, "scoped")
	s2a, _ := reg.GetWithContext(ctxA, "scoped")
	if s1a != s2a {
		t.Fatalf("scoped instances within same scope differ")
	}
	s1b, _ := reg.GetWithContext(ctxB, "scoped")
	if s1b == s1a {
		t.Fatalf("scoped instances across scopes should differ")
	}

	// Error path: unknown service
	_, err := reg.Get("missing")
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("expected ErrServiceNotFound, got %v", err)
	}
}

// TestScopedServiceRegistry_DefaultBehavior ensures default scope falls through paths.
func TestScopedServiceRegistry_DefaultBehavior(t *testing.T) {
	// Default scope is singleton per GetDefaultServiceScope.
	reg := NewServiceRegistry()
	counter := 0
	type demo struct{ N int }
	reg.Register("plain", func() *demo { counter++; return &demo{N: counter} })
	v1, _ := reg.Get("plain")
	v2, _ := reg.Get("plain")
	if v1 != v2 {
		t.Fatalf("expected same singleton instance by default scope; got different pointers")
	}
	if v1.(*demo).N != 1 || counter != 1 {
		t.Fatalf("factory should have been invoked exactly once; counter=%d", counter)
	}
}

// Minimal module for naming conflict tests.
type testSimpleModule struct{ name string }

func (m *testSimpleModule) Name() string                                     { return m.name }
func (m *testSimpleModule) Init(app Application) error                       { return nil }
func (m *testSimpleModule) Description() string                              { return "" }
func (m *testSimpleModule) Dependencies() []ServiceDependency                { return nil }
func (m *testSimpleModule) Config() any                                      { return nil }
func (m *testSimpleModule) Services() []ServiceProvider                      { return nil }
func (m *testSimpleModule) Start(ctx context.Context, app Application) error { return nil }
func (m *testSimpleModule) Stop(ctx context.Context, app Application) error  { return nil }
