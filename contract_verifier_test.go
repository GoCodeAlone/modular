package modular

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- Mock Reloadable modules for contract tests ---

// wellBehavedReloadable satisfies all reload contract rules.
type wellBehavedReloadable struct{}

func (w *wellBehavedReloadable) Reload(ctx context.Context, _ []ConfigChange) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
func (w *wellBehavedReloadable) CanReload() bool              { return true }
func (w *wellBehavedReloadable) ReloadTimeout() time.Duration { return 5 * time.Second }

// zeroTimeoutReloadable returns a zero timeout.
type zeroTimeoutReloadable struct{ wellBehavedReloadable }

func (z *zeroTimeoutReloadable) ReloadTimeout() time.Duration { return 0 }

// panickyReloadable panics when CanReload is called.
type panickyReloadable struct{ wellBehavedReloadable }

func (p *panickyReloadable) CanReload() bool { panic("boom") }

// reloadPanickyReloadable panics when Reload is called.
type reloadPanickyReloadable struct{ wellBehavedReloadable }

func (r *reloadPanickyReloadable) Reload(_ context.Context, _ []ConfigChange) error {
	panic("reload boom")
}

// --- Mock HealthProviders for contract tests ---

// wellBehavedHealthProvider returns a proper report and respects cancellation.
type wellBehavedHealthProvider struct{}

func (w *wellBehavedHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []HealthReport{
		{
			Module:    "test-module",
			Component: "test-component",
			Status:    StatusHealthy,
			Message:   "ok",
			CheckedAt: time.Now(),
		},
	}, nil
}

// emptyModuleHealthProvider returns a report with empty Module field.
type emptyModuleHealthProvider struct{}

func (e *emptyModuleHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []HealthReport{
		{
			Module:    "",
			Component: "comp",
			Status:    StatusHealthy,
			CheckedAt: time.Now(),
		},
	}, nil
}

// cancelIgnoringHealthProvider ignores context cancellation.
type cancelIgnoringHealthProvider struct{}

func (c *cancelIgnoringHealthProvider) HealthCheck(_ context.Context) ([]HealthReport, error) {
	return []HealthReport{
		{
			Module:    "mod",
			Component: "comp",
			Status:    StatusHealthy,
			CheckedAt: time.Now(),
		},
	}, nil
}

// panickyHealthProvider panics when HealthCheck is called.
type panickyHealthProvider struct{}

func (p *panickyHealthProvider) HealthCheck(_ context.Context) ([]HealthReport, error) {
	panic("health check boom")
}

// --- Tests ---

func TestContractVerifier_ReloadWellBehaved(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyReloadContract(&wellBehavedReloadable{})
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for well-behaved reloadable, got %d: %+v", len(violations), violations)
	}
}

func TestContractVerifier_ReloadZeroTimeout(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyReloadContract(&zeroTimeoutReloadable{})

	found := false
	for _, v := range violations {
		if v.Rule == "must-return-positive-timeout" && v.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation for zero timeout, got: %+v", violations)
	}
}

func TestContractVerifier_ReloadPanicsOnCanReload(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyReloadContract(&panickyReloadable{})

	found := false
	for _, v := range violations {
		if v.Rule == "can-reload-must-not-panic" && v.Severity == "warning" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation for panicky CanReload, got: %+v", violations)
	}
}

func TestContractVerifier_HealthWellBehaved(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyHealthContract(&wellBehavedHealthProvider{})
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for well-behaved health provider, got %d: %+v", len(violations), violations)
	}
}

func TestContractVerifier_HealthEmptyModule(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyHealthContract(&emptyModuleHealthProvider{})

	found := false
	for _, v := range violations {
		if v.Rule == "must-have-module-field" && v.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation for empty Module field, got: %+v", violations)
	}
}

func TestContractVerifier_HealthIgnoresCancellation(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyHealthContract(&cancelIgnoringHealthProvider{})

	found := false
	for _, v := range violations {
		if v.Rule == "must-respect-context-cancellation" && v.Severity == "warning" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation for ignoring cancellation, got: %+v", violations)
	}
}

// TestContractVerifier_ReloadPanicWrapsErrReloadPanic verifies that a panic inside
// Reload is captured and the resulting violation error wraps ErrReloadPanic.
func TestContractVerifier_ReloadPanicWrapsErrReloadPanic(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyReloadContract(&reloadPanickyReloadable{})

	found := false
	for _, v := range violations {
		if v.Rule == "reload-must-not-panic" && errors.Is(v.Err, ErrReloadPanic) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation with ErrReloadPanic, got: %+v", violations)
	}
}

// TestContractVerifier_HealthPanicWrapsErrHealthCheckPanic verifies that a panic inside
// HealthCheck is captured and the resulting violation error wraps ErrHealthCheckPanic.
func TestContractVerifier_HealthPanicWrapsErrHealthCheckPanic(t *testing.T) {
	verifier := NewStandardContractVerifier()
	violations := verifier.VerifyHealthContract(&panickyHealthProvider{})

	found := false
	for _, v := range violations {
		if v.Rule == "health-check-must-not-panic" && errors.Is(v.Err, ErrHealthCheckPanic) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected violation with ErrHealthCheckPanic, got: %+v", violations)
	}
}
