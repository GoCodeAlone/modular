package modular

import (
	"sync"
	"testing"
)

type testTenantAwareModule struct {
	name       string
	registered []TenantID
	removed    []TenantID
	mu         sync.Mutex
}

func (m *testTenantAwareModule) Name() string            { return m.name }
func (m *testTenantAwareModule) Init(Application) error  { return nil }
func (m *testTenantAwareModule) Start(Application) error { return nil }
func (m *testTenantAwareModule) Stop(Application) error  { return nil }
func (m *testTenantAwareModule) OnTenantRegistered(tid TenantID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registered = append(m.registered, tid)
}
func (m *testTenantAwareModule) OnTenantRemoved(tid TenantID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, tid)
}

type tsCaptureLogger struct {
	entries []string
	mu      sync.Mutex
}

func (l *tsCaptureLogger) record(msg string) {
	l.mu.Lock()
	l.entries = append(l.entries, msg)
	l.mu.Unlock()
}
func (l *tsCaptureLogger) Debug(msg string, _ ...interface{}) { l.record("DEBUG:" + msg) }
func (l *tsCaptureLogger) Info(msg string, _ ...interface{})  { l.record("INFO:" + msg) }
func (l *tsCaptureLogger) Warn(msg string, _ ...interface{})  { l.record("WARN:" + msg) }
func (l *tsCaptureLogger) Error(msg string, _ ...interface{}) { l.record("ERROR:" + msg) }
func (l *tsCaptureLogger) With(_ ...interface{}) Logger       { return l }

func TestStandardTenantService_RegisterAndMergeConfigs(t *testing.T) {
	log := &tsCaptureLogger{}
	svc := NewStandardTenantService(log)
	tenant := TenantID("t1")

	// initial register with one section
	if err := svc.RegisterTenant(tenant, map[string]ConfigProvider{"A": NewStdConfigProvider(&struct{ V int }{1})}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	// merge second call with different section (should not error)
	if err := svc.RegisterTenant(tenant, map[string]ConfigProvider{"B": NewStdConfigProvider(&struct{ V int }{2})}); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	cfgA, err := svc.GetTenantConfig(tenant, "A")
	if err != nil {
		t.Fatalf("missing A: %v", err)
	}
	if cfgA.GetConfig().(*struct{ V int }).V != 1 {
		t.Fatalf("unexpected A value")
	}
	cfgB, err := svc.GetTenantConfig(tenant, "B")
	if err != nil {
		t.Fatalf("missing B: %v", err)
	}
	if cfgB.GetConfig().(*struct{ V int }).V != 2 {
		t.Fatalf("unexpected B value")
	}
}

func TestStandardTenantService_ModuleNotificationsAndIdempotency(t *testing.T) {
	log := &tsCaptureLogger{}
	svc := NewStandardTenantService(log)
	m := &testTenantAwareModule{name: "mod1"}

	// Register module first, then tenants
	if err := svc.RegisterTenantAwareModule(m); err != nil {
		t.Fatalf("module reg failed: %v", err)
	}
	svc.RegisterTenant("t1", nil)
	svc.RegisterTenant("t2", nil)

	// Duplicate module registration should not duplicate notifications
	if err := svc.RegisterTenantAwareModule(m); err != nil {
		t.Fatalf("dup module reg failed: %v", err)
	}

	m.mu.Lock()
	regCount := len(m.registered)
	m.mu.Unlock()
	if regCount != 2 {
		t.Fatalf("expected 2 registrations, got %d", regCount)
	}
}

func TestStandardTenantService_ModuleSeesExistingTenantsOnRegister(t *testing.T) {
	log := &tsCaptureLogger{}
	svc := NewStandardTenantService(log)
	svc.RegisterTenant("t1", nil)
	svc.RegisterTenant("t2", nil)
	m := &testTenantAwareModule{name: "late"}
	if err := svc.RegisterTenantAwareModule(m); err != nil {
		t.Fatalf("late reg failed: %v", err)
	}
	m.mu.Lock()
	count := len(m.registered)
	m.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 notifications for existing tenants, got %d", count)
	}
}

func TestStandardTenantService_RemoveTenantNotifications(t *testing.T) {
	log := &tsCaptureLogger{}
	svc := NewStandardTenantService(log)
	m := &testTenantAwareModule{name: "mod"}
	svc.RegisterTenantAwareModule(m)
	svc.RegisterTenant("t1", nil)
	if err := svc.RemoveTenant("t1"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	m.mu.Lock()
	removedCount := len(m.removed)
	m.mu.Unlock()
	if removedCount != 1 {
		t.Fatalf("expected 1 removal notification, got %d", removedCount)
	}
}

func TestStandardTenantService_RegisterTenantConfigSection_CreatesTenantAndErrors(t *testing.T) {
	log := &tsCaptureLogger{}
	svc := NewStandardTenantService(log)

	// Attempt with nil provider (error)
	if err := svc.RegisterTenantConfigSection("tX", "Section", nil); err == nil {
		t.Fatalf("expected error for nil provider")
	}

	// Now valid provider should create tenant implicitly
	if err := svc.RegisterTenantConfigSection("tX", "Section", NewStdConfigProvider(&struct{ V string }{"ok"})); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := svc.GetTenantConfig("tX", "Section"); err != nil {
		t.Fatalf("expected section: %v", err)
	}
}

func TestStandardTenantService_logTenantConfigStatus_EdgeCases(t *testing.T) {
	log := &captureLogger{}
	svc := NewStandardTenantService(log)
	// Unregistered tenant (warn path)
	svc.logTenantConfigStatus("absent")
	// Register tenant and add two sections, then log
	svc.RegisterTenant("t1", map[string]ConfigProvider{"A": NewStdConfigProvider(&struct{ X int }{1}), "B": NewStdConfigProvider(&struct{ Y int }{2})})
	svc.logTenantConfigStatus("t1")
}
