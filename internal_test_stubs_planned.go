//go:build planned

package modular

// Internal test stubs for planned tests - T001-T030
// These are minimal interface/type stubs to ensure tests compile without missing symbols

// ReloadManager interface for reload functionality tests
type ReloadManager interface {
	// Reload triggers a configuration reload
	Reload() error
	// IsReloading returns true if a reload is in progress
	IsReloading() bool
	// RegisterReloadCallback registers a callback for reload events
	RegisterReloadCallback(callback func() error) error
}

// HealthChecker interface for health check functionality tests
type HealthChecker interface {
	// Check performs a health check
	Check() error
	// IsHealthy returns the current health status
	IsHealthy() bool
	// SetInterval sets the health check interval
	SetInterval(interval int) error
}

// ServiceScope represents service scope information
type ServiceScope struct {
	Name     string
	Services []string
	Tenant   string
}

// TenantGuard interface for tenant isolation tests
type TenantGuard interface {
	// EnforceIsolation enforces tenant isolation
	EnforceIsolation(tenantID string) error
	// CheckCrossAccess checks for cross-tenant access
	CheckCrossAccess(fromTenant, toTenant string) error
}

// DecoratorConfig represents decorator configuration
type DecoratorConfig struct {
	Name     string
	Priority int
	Type     string
}

// ErrorTaxonomy represents error classification
type ErrorTaxonomy struct {
	Category string
	Code     string
	Message  string
}

// SecretRedactor interface for secret redaction tests
type SecretRedactor interface {
	// RedactSecrets redacts secrets from logs
	RedactSecrets(data string) string
	// TrackProvenance tracks secret provenance
	TrackProvenance(secret, source string) error
}

// SchedulerPolicy interface for scheduler tests  
type SchedulerPolicy interface {
	// GetCatchUpPolicy returns the catch-up policy
	GetCatchUpPolicy() string
	// SetBoundedCatchUp sets bounded catch-up policy
	SetBoundedCatchUp(bound int) error
}

// ACMEEscalation represents ACME escalation events
type ACMEEscalation struct {
	EventType string
	Domain    string
	Error     error
}

// OIDCProvider interface for OIDC tests
type OIDCProvider interface {
	// GetProviderName returns the provider name
	GetProviderName() string
	// Authenticate performs authentication
	Authenticate(token string) error
}

// AuthMechanism interface for auth mechanism tests
type AuthMechanism interface {
	// GetType returns the auth mechanism type
	GetType() string
	// Validate validates authentication
	Validate(credentials interface{}) error
}

// MetricsEmitter interface for metrics tests
type MetricsEmitter interface {
	// EmitReloadMetric emits reload metrics
	EmitReloadMetric(duration int64) error
	// EmitHealthMetric emits health metrics
	EmitHealthMetric(status string) error
}

// TestApplicationStub provides a stub Application for testing
type TestApplicationStub struct{}

func (t *TestApplicationStub) ConfigProvider() ConfigProvider { return nil }
func (t *TestApplicationStub) SvcRegistry() ServiceRegistry   { return nil }
func (t *TestApplicationStub) Logger() Logger                 { return nil }
func (t *TestApplicationStub) RegisterModule(module Module) error { return nil }
func (t *TestApplicationStub) Run() error                     { return nil }
func (t *TestApplicationStub) Start() error                   { return nil }
func (t *TestApplicationStub) Stop() error                    { return nil }
func (t *TestApplicationStub) AwaitShutdown() error           { return nil }