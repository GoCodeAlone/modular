package modular

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TenantGuardMode controls how the tenant guard responds to violations.
type TenantGuardMode int

const (
	// TenantGuardStrict blocks the operation and returns an error on violation.
	TenantGuardStrict TenantGuardMode = iota
	// TenantGuardLenient logs the violation but allows the operation to proceed.
	TenantGuardLenient
	// TenantGuardDisabled performs no validation at all.
	TenantGuardDisabled
)

// String returns the string representation of a TenantGuardMode.
func (m TenantGuardMode) String() string {
	switch m {
	case TenantGuardStrict:
		return "strict"
	case TenantGuardLenient:
		return "lenient"
	case TenantGuardDisabled:
		return "disabled"
	default:
		return fmt.Sprintf("unknown(%d)", int(m))
	}
}

// ViolationType categorizes the kind of tenant boundary violation.
type ViolationType int

const (
	// CrossTenant indicates an attempt to access another tenant's resources.
	CrossTenant ViolationType = iota
	// InvalidContext indicates the tenant context is malformed or invalid.
	InvalidContext
	// MissingContext indicates no tenant context was provided.
	MissingContext
	// Unauthorized indicates the caller lacks permission for the tenant operation.
	Unauthorized
)

// String returns the string representation of a ViolationType.
func (v ViolationType) String() string {
	switch v {
	case CrossTenant:
		return "cross_tenant"
	case InvalidContext:
		return "invalid_context"
	case MissingContext:
		return "missing_context"
	case Unauthorized:
		return "unauthorized"
	default:
		return fmt.Sprintf("unknown(%d)", int(v))
	}
}

// Severity indicates the severity level of a tenant violation.
type Severity int

const (
	// SeverityLow indicates a minor violation.
	SeverityLow Severity = iota
	// SeverityMedium indicates a moderate violation.
	SeverityMedium
	// SeverityHigh indicates a serious violation.
	SeverityHigh
	// SeverityCritical indicates a critical violation requiring immediate attention.
	SeverityCritical
)

// String returns the string representation of a Severity.
func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// TenantViolation represents a detected tenant boundary violation.
type TenantViolation struct {
	Type      ViolationType
	Severity  Severity
	TenantID  string
	TargetID  string
	Timestamp time.Time
	Details   string
}

// TenantGuard validates tenant access and tracks violations.
type TenantGuard interface {
	// GetMode returns the current guard mode.
	GetMode() TenantGuardMode

	// ValidateAccess checks whether the given violation should be blocked.
	// In Strict mode, it returns an error. In Lenient mode, it records the
	// violation but returns nil. In Disabled mode, it is a no-op.
	ValidateAccess(ctx context.Context, violation TenantViolation) error

	// GetRecentViolations returns a deep copy of recent violations, ordered oldest-first.
	GetRecentViolations() []TenantViolation
}

// TenantGuardConfig holds configuration for a StandardTenantGuard.
type TenantGuardConfig struct {
	Mode          TenantGuardMode
	Whitelist     map[string][]string // tenantID -> allowed target IDs
	MaxViolations int                 // ring buffer capacity, default 1000
	LogViolations bool                // whether to log violations, default true
}

// DefaultTenantGuardConfig returns a TenantGuardConfig with sensible defaults.
func DefaultTenantGuardConfig() TenantGuardConfig {
	return TenantGuardConfig{
		Mode:          TenantGuardStrict,
		Whitelist:     make(map[string][]string),
		MaxViolations: 1000,
		LogViolations: true,
	}
}

// TenantGuardOption is a functional option for configuring a StandardTenantGuard.
type TenantGuardOption func(*StandardTenantGuard)

// WithTenantGuardLogger sets a structured logger on the guard.
func WithTenantGuardLogger(l Logger) TenantGuardOption {
	return func(g *StandardTenantGuard) {
		g.logger = l
	}
}

// WithTenantGuardSubject sets a Subject for event emission on the guard.
func WithTenantGuardSubject(s Subject) TenantGuardOption {
	return func(g *StandardTenantGuard) {
		g.subject = s
	}
}

// StandardTenantGuard is the default TenantGuard implementation.
// It uses a ring buffer to store recent violations and optionally emits
// CloudEvents when violations are detected.
type StandardTenantGuard struct {
	config     TenantGuardConfig
	whitelist  map[string]map[string]struct{} // deep-copied set for fast lookups
	violations []TenantViolation
	head       int
	count      int
	mu         sync.RWMutex
	logger     Logger
	subject    Subject
}

// NewStandardTenantGuard creates a new StandardTenantGuard with the given config and options.
// The whitelist is deep-copied and converted to a set for safe, fast lookups.
func NewStandardTenantGuard(config TenantGuardConfig, opts ...TenantGuardOption) *StandardTenantGuard {
	if config.MaxViolations <= 0 {
		config.MaxViolations = 1000
	}

	// Deep-copy and convert whitelist to set
	wl := make(map[string]map[string]struct{}, len(config.Whitelist))
	for tenant, targets := range config.Whitelist {
		set := make(map[string]struct{}, len(targets))
		for _, t := range targets {
			set[t] = struct{}{}
		}
		wl[tenant] = set
	}

	g := &StandardTenantGuard{
		config:     config,
		whitelist:  wl,
		violations: make([]TenantViolation, config.MaxViolations),
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// GetMode returns the current guard mode.
func (g *StandardTenantGuard) GetMode() TenantGuardMode {
	return g.config.Mode
}

// ValidateAccess checks the violation against the guard's policy.
func (g *StandardTenantGuard) ValidateAccess(ctx context.Context, violation TenantViolation) error {
	if g.config.Mode == TenantGuardDisabled {
		return nil
	}

	// Set timestamp if not provided
	if violation.Timestamp.IsZero() {
		violation.Timestamp = time.Now()
	}

	// Check whitelist (set-based O(1) lookup)
	if targets, ok := g.whitelist[violation.TenantID]; ok {
		if _, allowed := targets[violation.TargetID]; allowed {
			return nil
		}
	}

	// Record violation
	g.mu.Lock()
	g.addViolation(violation)
	g.mu.Unlock()

	// Log if configured
	if g.config.LogViolations && g.logger != nil {
		g.logger.Warn("Tenant violation detected",
			"type", violation.Type.String(),
			"severity", violation.Severity.String(),
			"tenant", violation.TenantID,
			"target", violation.TargetID,
			"details", violation.Details,
		)
	}

	// Emit event using NewCloudEvent helper (sets ID, specversion, time)
	if g.subject != nil {
		event := NewCloudEvent(EventTypeTenantViolation, "com.modular.tenant.guard", violation, nil)
		_ = g.subject.NotifyObservers(ctx, event)
	}

	// In strict mode, return error
	if g.config.Mode == TenantGuardStrict {
		return ErrTenantIsolationViolation
	}

	// Lenient mode: violation recorded, but allow the operation
	return nil
}

// GetRecentViolations returns a deep copy of recent violations ordered oldest-first.
func (g *StandardTenantGuard) GetRecentViolations() []TenantViolation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.count == 0 {
		return nil
	}

	result := make([]TenantViolation, g.count)
	max := g.config.MaxViolations

	if g.count < max {
		// Buffer not yet full — entries are at indices 0..count-1
		copy(result, g.violations[:g.count])
	} else {
		// Buffer full — oldest is at head, wrap around
		oldest := g.head % max
		n := copy(result, g.violations[oldest:])
		copy(result[n:], g.violations[:oldest])
	}

	return result
}

// addViolation writes a violation into the ring buffer.
// Caller must hold the write lock.
func (g *StandardTenantGuard) addViolation(v TenantViolation) {
	max := g.config.MaxViolations
	g.violations[g.head%max] = v
	g.head++
	if g.count < max {
		g.count++
	}
}
