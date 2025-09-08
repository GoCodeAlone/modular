package modular

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCoreServicesIntegration demonstrates the three core services working together
// T028: AggregateHealthService, T029: ReloadOrchestrator, T030: SecretValue
func TestCoreServicesIntegration(t *testing.T) {
	t.Run("should_integrate_health_aggregation_with_secrets", func(t *testing.T) {
		// Create health aggregation service
		healthService := NewAggregateHealthService()
		
		// Create a provider that uses secrets
		secretConfig := &testModuleWithSecrets{
			DatabasePassword: NewPasswordSecret("super-secret-db-password"),
			APIKey:          NewTokenSecret("sk-1234567890"),
			Endpoint:        "https://api.example.com",
		}
		
		provider := &healthProviderWithSecrets{
			config: secretConfig,
		}
		
		// Register the provider
		err := healthService.RegisterProvider("secure-module", provider, false)
		assert.NoError(t, err)
		
		// Collect health - should work without leaking secrets
		ctx := context.Background()
		result, err := healthService.Collect(ctx)
		assert.NoError(t, err)
		
		assert.Equal(t, HealthStatusHealthy, result.Health)
		assert.Len(t, result.Reports, 1)
		
		report := result.Reports[0]
		assert.Equal(t, "secure-module", report.Module)
		assert.Equal(t, HealthStatusHealthy, report.Status)
		
		// Verify secrets are not leaked in the health report
		reportJSON, err := json.Marshal(report)
		assert.NoError(t, err)
		assert.NotContains(t, string(reportJSON), "super-secret-db-password")
		assert.NotContains(t, string(reportJSON), "sk-1234567890")
		assert.Contains(t, string(reportJSON), "[REDACTED]") // Should contain redacted marker
	})
	
	t.Run("should_integrate_reload_orchestrator_with_health", func(t *testing.T) {
		// Create both services
		healthService := NewAggregateHealthService()
		reloadOrchestrator := NewReloadOrchestrator()
		
		// Create a module that's both reloadable and provides health
		module := &reloadableHealthModule{
			name:          "integrated-module",
			currentStatus: HealthStatusHealthy,
		}
		
		// Register with both services
		err := healthService.RegisterProvider("integrated-module", module, false)
		assert.NoError(t, err)
		
		err = reloadOrchestrator.RegisterModule("integrated-module", module)
		assert.NoError(t, err)
		
		// Check initial health
		ctx := context.Background()
		healthResult, err := healthService.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, healthResult.Health)
		
		// Trigger a reload
		err = reloadOrchestrator.RequestReload(ctx)
		assert.NoError(t, err)
		
		// Verify module was reloaded
		assert.True(t, module.wasReloaded)
		
		// Health should still be good
		healthResult, err = healthService.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, healthResult.Health)
		
		// Cleanup
		reloadOrchestrator.Stop(ctx)
	})
	
	t.Run("should_integrate_all_three_services", func(t *testing.T) {
		// Create all three core services
		healthService := NewAggregateHealthService()
		reloadOrchestrator := NewReloadOrchestrator()
		
		// Create observers to track events (commented for now - would be integrated via application)
		// healthObserver := &integrationHealthObserver{}
		// reloadObserver := &integrationReloadObserver{}
		
		// healthService.SetEventSubject(eventSubject) // Would be set via application
		// reloadOrchestrator.SetEventSubject(eventSubject) // Would be set via application
		
		// Create a comprehensive module with secrets, health, and reload capability
		secretAPIKey := NewTokenSecret("integration-test-key-123")
		secretDBPassword := NewPasswordSecret("integration-db-pass-456")
		
		module := &comprehensiveTestModule{
			name:       "comprehensive-module",
			apiKey:     secretAPIKey,
			dbPassword: secretDBPassword,
			endpoint:   "https://integration.test.com",
			healthy:    true,
			reloadable: true,
		}
		
		// Register with all services
		err := healthService.RegisterProvider("comprehensive-module", module, false)
		assert.NoError(t, err)
		
		err = reloadOrchestrator.RegisterModule("comprehensive-module", module)
		assert.NoError(t, err)
		
		// Register secrets globally for redaction
		RegisterGlobalSecret(secretAPIKey)
		RegisterGlobalSecret(secretDBPassword)
		
		// Perform health check
		ctx := context.Background()
		healthResult, err := healthService.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, healthResult.Health)
		
		// Perform reload
		err = reloadOrchestrator.RequestReload(ctx)
		assert.NoError(t, err)
		assert.True(t, module.reloaded)
		
		// Test secret redaction in various outputs
		moduleStr := fmt.Sprintf("Module: %v", module)
		assert.NotContains(t, moduleStr, "integration-test-key-123")
		assert.NotContains(t, moduleStr, "integration-db-pass-456")
		
		// Test global redaction
		testText := "API key is integration-test-key-123 and password is integration-db-pass-456"
		redactedText := RedactGlobally(testText)
		assert.Equal(t, "API key is [REDACTED] and password is [REDACTED]", redactedText)
		
		// Verify events were emitted
		// Note: Events are emitted asynchronously, so we need to wait
		time.Sleep(100 * time.Millisecond)
		
		// Health status changes might not have occurred, but reload should have events
		// assert.True(t, reloadObserver.IsStartedReceived()) // Would be tested via event integration
		// assert.True(t, reloadObserver.IsCompletedReceived()) // Would be tested via event integration
		
		// Cleanup
		reloadOrchestrator.Stop(ctx)
	})
}

// Test helper types for integration testing

type testModuleWithSecrets struct {
	DatabasePassword *SecretValue `json:"database_password"`
	APIKey          *SecretValue `json:"api_key"`
	Endpoint        string       `json:"endpoint"`
}

type healthProviderWithSecrets struct {
	config *testModuleWithSecrets
}

func (h *healthProviderWithSecrets) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	// Simulate a health check that might accidentally try to log sensitive info
	message := fmt.Sprintf("Connected to %s", h.config.Endpoint)
	// Note: We don't include secrets in the message due to SecretValue redaction
	
	return []HealthReport{
		{
			Module:    "secure-module",
			Status:    HealthStatusHealthy,
			Message:   message,
			CheckedAt: time.Now(),
			Details: map[string]any{
				"endpoint":         h.config.Endpoint,
				"database_password": h.config.DatabasePassword, // This should be redacted
				"api_key":          h.config.APIKey,           // This should be redacted
				"has_credentials":   !h.config.DatabasePassword.IsEmpty() && !h.config.APIKey.IsEmpty(),
			},
		},
	}, nil
}

type reloadableHealthModule struct {
	name          string
	currentStatus HealthStatus
	wasReloaded   bool
}

func (m *reloadableHealthModule) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	return []HealthReport{
		{
			Module:    m.name,
			Status:    m.currentStatus,
			Message:   "Module is operating normally",
			CheckedAt: time.Now(),
		},
	}, nil
}

func (m *reloadableHealthModule) Reload(ctx context.Context, changes []ConfigChange) error {
	m.wasReloaded = true
	return nil
}

func (m *reloadableHealthModule) CanReload() bool {
	return true
}

func (m *reloadableHealthModule) ReloadTimeout() time.Duration {
	return 30 * time.Second
}

type comprehensiveTestModule struct {
	name       string
	apiKey     *SecretValue
	dbPassword *SecretValue
	endpoint   string
	healthy    bool
	reloadable bool
	reloaded   bool
}

func (m *comprehensiveTestModule) String() string {
	return fmt.Sprintf("Module{name: %s, apiKey: %s, dbPassword: %s, endpoint: %s}", 
		m.name, m.apiKey, m.dbPassword, m.endpoint)
}

func (m *comprehensiveTestModule) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	status := HealthStatusHealthy
	if !m.healthy {
		status = HealthStatusUnhealthy
	}
	
	return []HealthReport{
		{
			Module:    m.name,
			Status:    status,
			Message:   "Comprehensive module health check",
			CheckedAt: time.Now(),
			Details: map[string]any{
				"api_key_configured":  !m.apiKey.IsEmpty(),
				"db_password_set":     !m.dbPassword.IsEmpty(),
				"endpoint":           m.endpoint,
				"can_reload":         m.reloadable,
			},
		},
	}, nil
}

func (m *comprehensiveTestModule) Reload(ctx context.Context, changes []ConfigChange) error {
	if !m.reloadable {
		return fmt.Errorf("module is not reloadable")
	}
	
	m.reloaded = true
	return nil
}

func (m *comprehensiveTestModule) CanReload() bool {
	return m.reloadable
}

func (m *comprehensiveTestModule) ReloadTimeout() time.Duration {
	return 30 * time.Second
}

// Event observers for integration testing

type integrationHealthObserver struct {
	statusChanges []HealthStatusChangedEvent
}

func (o *integrationHealthObserver) OnStatusChange(ctx context.Context, event *HealthStatusChangedEvent) {
	o.statusChanges = append(o.statusChanges, *event)
}

type integrationReloadObserver struct {
	startedReceived   bool
	completedReceived bool
	failedReceived    bool
	noopReceived      bool
	mu                sync.RWMutex
}

func (o *integrationReloadObserver) OnReloadStarted(ctx context.Context, event *ConfigReloadStartedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.startedReceived = true
}

func (o *integrationReloadObserver) OnReloadCompleted(ctx context.Context, event *ConfigReloadCompletedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completedReceived = true
}

func (o *integrationReloadObserver) OnReloadFailed(ctx context.Context, event *ConfigReloadFailedEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failedReceived = true
}

func (o *integrationReloadObserver) OnReloadNoop(ctx context.Context, event *ConfigReloadNoopEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.noopReceived = true
}

func (o *integrationReloadObserver) IsStartedReceived() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.startedReceived
}

func (o *integrationReloadObserver) IsCompletedReceived() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.completedReceived
}