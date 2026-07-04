package modular

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type blockingTenantAwareModule struct {
	entered chan struct{}
	release chan struct{}
}

func newBlockingTenantAwareModule() *blockingTenantAwareModule {
	return &blockingTenantAwareModule{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (m *blockingTenantAwareModule) Name() string                          { return "blockingTenantAwareModule" }
func (m *blockingTenantAwareModule) Init(Application) error                { return nil }
func (m *blockingTenantAwareModule) Start(context.Context) error           { return nil }
func (m *blockingTenantAwareModule) Stop(context.Context) error            { return nil }
func (m *blockingTenantAwareModule) Dependencies() []string                { return nil }
func (m *blockingTenantAwareModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *blockingTenantAwareModule) RequiresServices() []ServiceDependency { return nil }
func (m *blockingTenantAwareModule) RegisterConfig(Application)            {}

func (m *blockingTenantAwareModule) OnTenantRegistered(TenantID) {
	m.block()
}

func (m *blockingTenantAwareModule) OnTenantRemoved(TenantID) {
	m.block()
}

func (m *blockingTenantAwareModule) block() {
	select {
	case <-m.entered:
	default:
		close(m.entered)
	}
	<-m.release
}

func TestTenantLifecycleCallbacksDoNotStarveTenantReaders(t *testing.T) {
	for _, tc := range []struct {
		name     string
		call     func(*StandardTenantService, TenantID) error
		readable TenantID
	}{
		{
			name: "RegisterTenant",
			call: func(ts *StandardTenantService, tenantID TenantID) error {
				return ts.RegisterTenant(tenantID, map[string]ConfigProvider{
					"app": NewStdConfigProvider(&struct{ Enabled bool }{Enabled: true}),
				})
			},
			readable: TenantID("tenant-under-registration"),
		},
		{
			name: "RemoveTenant",
			call: func(ts *StandardTenantService, tenantID TenantID) error {
				return ts.RemoveTenant(tenantID)
			},
			readable: TenantID("existing-tenant"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := NewStandardTenantService(slog.Default())
			module := newBlockingTenantAwareModule()
			if err := ts.RegisterTenantAwareModule(module); err != nil {
				t.Fatalf("register module: %v", err)
			}
			if tc.name == "RemoveTenant" {
				tenantCfg := NewTenantConfigProvider(nil)
				tenantCfg.SetTenantConfig(TenantID("existing-tenant"), "app", NewStdConfigProvider(&struct{ Enabled bool }{Enabled: true}))
				ts.tenantConfigs[TenantID("existing-tenant")] = tenantCfg
				ts.moduleNotifications[module] = map[TenantID]bool{TenantID("existing-tenant"): true}
			}

			errCh := make(chan error, 1)
			go func() {
				errCh <- tc.call(ts, tc.readable)
			}()

			select {
			case <-module.entered:
			case <-time.After(time.Second):
				close(module.release)
				t.Fatal("lifecycle call did not reach tenant-aware callback")
			}

			readDone := make(chan struct{})
			go func() {
				_, _ = ts.GetTenantConfig(tc.readable, "app")
				close(readDone)
			}()

			select {
			case <-readDone:
			case <-time.After(100 * time.Millisecond):
				close(module.release)
				t.Fatal("tenant config read blocked behind tenant-aware callback")
			}

			close(module.release)
			if err := <-errCh; err != nil {
				t.Fatalf("lifecycle call: %v", err)
			}
		})
	}
}

func TestRegisterTenantConfigSectionRejectsNilProviderWithoutUnlockPanic(t *testing.T) {
	ts := NewStandardTenantService(slog.Default())

	err := ts.RegisterTenantConfigSection(TenantID("tenant-a"), "app", nil)
	if !errors.Is(err, ErrTenantRegisterNilConfig) {
		t.Fatalf("expected ErrTenantRegisterNilConfig, got %v", err)
	}

	done := make(chan struct{})
	go func() {
		_, _ = ts.GetTenantConfig(TenantID("tenant-a"), "app")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("tenant service mutex remained locked after nil provider error")
	}
}
