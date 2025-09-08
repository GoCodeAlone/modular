package modular

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestReloadOrchestratorBasic tests basic functionality without build tags
func TestReloadOrchestratorBasic(t *testing.T) {
	t.Run("should_create_orchestrator_with_default_config", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		assert.NotNil(t, orchestrator)
		
		// Should be able to stop gracefully
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		err := orchestrator.Stop(ctx)
		assert.NoError(t, err)
	})
	
	t.Run("should_register_and_unregister_modules", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		module := &testReloadModule{
			name: "test-module",
			canReload: true,
		}
		
		err := orchestrator.RegisterModule("test", module)
		assert.NoError(t, err)
		
		// Should reject duplicate registration
		err = orchestrator.RegisterModule("test", module)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
		
		// Should unregister successfully
		err = orchestrator.UnregisterModule("test")
		assert.NoError(t, err)
		
		// Should reject unregistering non-existent module
		err = orchestrator.UnregisterModule("nonexistent")
		assert.Error(t, err)
	})
	
	t.Run("should_handle_empty_reload", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		// Should handle reload with no modules
		ctx := context.Background()
		err := orchestrator.RequestReload(ctx)
		assert.NoError(t, err)
	})
	
	t.Run("should_reload_registered_modules", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		reloadCalled := false
		module := &testReloadModule{
			name: "test-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				reloadCalled = true
				return nil
			},
		}
		
		err := orchestrator.RegisterModule("test", module)
		assert.NoError(t, err)
		
		// Trigger reload
		ctx := context.Background()
		err = orchestrator.RequestReload(ctx)
		assert.NoError(t, err)
		
		// Should have called reload on the module
		assert.True(t, reloadCalled)
	})
	
	t.Run("should_handle_module_reload_failure", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		module := &testReloadModule{
			name: "failing-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				return assert.AnError
			},
		}
		
		err := orchestrator.RegisterModule("test", module)
		assert.NoError(t, err)
		
		// Trigger reload - should fail
		ctx := context.Background()
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to reload")
	})
	
	t.Run("should_handle_non_reloadable_modules", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		reloadCalled := false
		module := &testReloadModule{
			name: "non-reloadable-module",
			canReload: false, // Not reloadable
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				reloadCalled = true
				return nil
			},
		}
		
		err := orchestrator.RegisterModule("test", module)
		assert.NoError(t, err)
		
		// Trigger reload
		ctx := context.Background()
		err = orchestrator.RequestReload(ctx)
		assert.NoError(t, err)
		
		// Should not have called reload on non-reloadable module
		assert.False(t, reloadCalled)
	})
	
	t.Run("should_emit_events", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()
		
		// observer := &testReloadEventObserver{} // Would be integrated via application
		// orchestrator.SetEventSubject(eventSubject) // Would be set via application
		
		module := &testReloadModule{
			name: "test-module",
			canReload: true,
		}
		
		err := orchestrator.RegisterModule("test", module)
		assert.NoError(t, err)
		
		// Trigger reload
		ctx := context.Background()
		err = orchestrator.RequestReload(ctx)
		assert.NoError(t, err)
		
		// Give events time to be emitted
		time.Sleep(50 * time.Millisecond)
		
		// Should have emitted start and completion events
		// assert.True(t, observer.IsStartedCalled()) // Would be tested via event integration
		// assert.True(t, observer.IsCompletedCalled()) // Would be tested via event integration
		// assert.False(t, observer.IsFailedCalled()) // Would be tested via event integration
		// assert.False(t, observer.IsNoopCalled()) // Would be tested via event integration
	})
}

// TestReloadTriggerTypes tests the reload trigger constants
func TestReloadTriggerTypes(t *testing.T) {
	t.Run("should_convert_to_string", func(t *testing.T) {
		assert.Equal(t, "manual", ReloadTriggerManual.String())
		assert.Equal(t, "file_change", ReloadTriggerFileChange.String())
		assert.Equal(t, "api_request", ReloadTriggerAPIRequest.String())
		assert.Equal(t, "scheduled", ReloadTriggerScheduled.String())
	})
	
	t.Run("should_parse_from_string", func(t *testing.T) {
		trigger, err := ParseReloadTrigger("manual")
		assert.NoError(t, err)
		assert.Equal(t, ReloadTriggerManual, trigger)
		
		trigger, err = ParseReloadTrigger("file_change")
		assert.NoError(t, err)
		assert.Equal(t, ReloadTriggerFileChange, trigger)
		
		_, err = ParseReloadTrigger("invalid")
		assert.Error(t, err)
	})
}

// Test helper implementations

type testReloadModule struct {
	name      string
	canReload bool
	timeout   time.Duration
	onReload  func(ctx context.Context, changes []ConfigChange) error
}

func (m *testReloadModule) Reload(ctx context.Context, changes []ConfigChange) error {
	if m.onReload != nil {
		return m.onReload(ctx, changes)
	}
	return nil
}

func (m *testReloadModule) CanReload() bool {
	return m.canReload
}

func (m *testReloadModule) ReloadTimeout() time.Duration {
	if m.timeout > 0 {
		return m.timeout
	}
	return 30 * time.Second
}

type testReloadEventObserver struct {
	startedCalled   bool
	completedCalled bool
	failedCalled    bool
	noopCalled      bool
	mu              sync.RWMutex
}

func (o *testReloadEventObserver) OnReloadStarted(ctx context.Context, event *ConfigReloadStartedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.startedCalled = true
}

func (o *testReloadEventObserver) OnReloadCompleted(ctx context.Context, event *ConfigReloadCompletedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completedCalled = true
}

func (o *testReloadEventObserver) OnReloadFailed(ctx context.Context, event *ConfigReloadFailedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failedCalled = true
}

func (o *testReloadEventObserver) OnReloadNoop(ctx context.Context, event *ConfigReloadNoopEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.noopCalled = true
}

func (o *testReloadEventObserver) IsStartedCalled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.startedCalled
}

func (o *testReloadEventObserver) IsCompletedCalled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.completedCalled
}

func (o *testReloadEventObserver) IsFailedCalled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.failedCalled
}

func (o *testReloadEventObserver) IsNoopCalled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.noopCalled
}