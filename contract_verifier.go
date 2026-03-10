package modular

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ContractViolation describes a single violation found during contract verification.
type ContractViolation struct {
	Contract    string // "reload" or "health"
	Rule        string // e.g., "must-return-positive-timeout"
	Description string
	Severity    string // "error" or "warning"
}

// ContractVerifier verifies that implementations of Reloadable and HealthProvider
// satisfy their behavioral contracts beyond what the type system enforces.
type ContractVerifier interface {
	VerifyReloadContract(module Reloadable) []ContractViolation
	VerifyHealthContract(provider HealthProvider) []ContractViolation
}

// StandardContractVerifier is the default implementation of ContractVerifier.
type StandardContractVerifier struct{}

// NewStandardContractVerifier creates a new StandardContractVerifier.
func NewStandardContractVerifier() *StandardContractVerifier {
	return &StandardContractVerifier{}
}

// VerifyReloadContract checks that a Reloadable module satisfies its behavioral contract:
//  1. ReloadTimeout() returns a positive duration
//  2. CanReload() is safe to call concurrently (no panics)
//  3. Reload() with empty changes is idempotent
//  4. Reload() respects context cancellation
func (v *StandardContractVerifier) VerifyReloadContract(module Reloadable) []ContractViolation {
	var violations []ContractViolation

	// 1. ReloadTimeout must return a positive duration.
	if timeout := module.ReloadTimeout(); timeout <= 0 {
		violations = append(violations, ContractViolation{
			Contract:    "reload",
			Rule:        "must-return-positive-timeout",
			Description: fmt.Sprintf("ReloadTimeout() returned %v, must be > 0", timeout),
			Severity:    "error",
		})
	}

	// 2. CanReload must be safe to call concurrently (no panics).
	if panicked := v.checkCanReloadConcurrency(module); panicked {
		violations = append(violations, ContractViolation{
			Contract:    "reload",
			Rule:        "can-reload-must-not-panic",
			Description: "CanReload() panicked during concurrent invocation",
			Severity:    "warning",
		})
	}

	// 3. Reload with empty changes should be idempotent.
	if err := v.checkReloadIdempotent(module); err != nil {
		violations = append(violations, ContractViolation{
			Contract:    "reload",
			Rule:        "empty-reload-must-be-idempotent",
			Description: fmt.Sprintf("Reload() with empty changes failed: %v", err),
			Severity:    "warning",
		})
	}

	// 4. Reload must respect context cancellation.
	if !v.checkReloadRespectsCancel(module) {
		violations = append(violations, ContractViolation{
			Contract:    "reload",
			Rule:        "must-respect-context-cancellation",
			Description: "Reload() with cancelled context did not return an error",
			Severity:    "warning",
		})
	}

	return violations
}

// checkCanReloadConcurrency calls CanReload 100 times concurrently and reports
// whether any invocation panicked.
func (v *StandardContractVerifier) checkCanReloadConcurrency(module Reloadable) bool {
	var (
		wg       sync.WaitGroup
		panicked int32
		mu       sync.Mutex
	)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					panicked = 1
					mu.Unlock()
				}
			}()
			module.CanReload()
		}()
	}
	wg.Wait()
	return panicked != 0
}

// checkReloadIdempotent calls Reload with empty changes twice and returns an error
// if either call fails or hangs beyond the timeout. Each call is guarded by a
// goroutine so a misbehaving module cannot block the verifier indefinitely.
func (v *StandardContractVerifier) checkReloadIdempotent(module Reloadable) error {
	for i, label := range []string{"first", "second"} {
		_ = i
		if err := v.runReloadWithGuard(module, label); err != nil {
			return err
		}
	}
	return nil
}

// runReloadWithGuard runs module.Reload in a goroutine and returns an error if
// it fails or exceeds the 5-second timeout.
func (v *StandardContractVerifier) runReloadWithGuard(module Reloadable, label string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: module.Reload(ctx, nil)}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("%s call: %w", label, r.err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s call: timed out waiting for Reload to return", label)
	}
}

// checkReloadRespectsCancel calls Reload with an already-cancelled context and
// returns true if Reload returned an error (i.e., it respected the cancellation).
func (v *StandardContractVerifier) checkReloadRespectsCancel(module Reloadable) bool {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := module.Reload(ctx, nil)
	return err != nil
}

// VerifyHealthContract checks that a HealthProvider satisfies its behavioral contract:
//  1. HealthCheck returns within 5 seconds
//  2. Reports have non-empty Module field
//  3. Reports have non-empty Component field
//  4. HealthCheck with cancelled context returns an error
func (v *StandardContractVerifier) VerifyHealthContract(provider HealthProvider) []ContractViolation {
	var violations []ContractViolation

	// 1 + 2 + 3: Check that HealthCheck returns in time and reports have required fields.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		reports []HealthReport
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		reports, err := provider.HealthCheck(ctx)
		ch <- result{reports, err}
	}()

	select {
	case <-ctx.Done():
		violations = append(violations, ContractViolation{
			Contract:    "health",
			Rule:        "must-return-within-timeout",
			Description: "HealthCheck() did not return within 5 seconds",
			Severity:    "error",
		})
		// Can't check fields if we timed out.
		return violations
	case res := <-ch:
		if res.err == nil {
			for _, report := range res.reports {
				if report.Module == "" {
					violations = append(violations, ContractViolation{
						Contract:    "health",
						Rule:        "must-have-module-field",
						Description: "HealthReport has empty Module field",
						Severity:    "error",
					})
				}
				if report.Component == "" {
					violations = append(violations, ContractViolation{
						Contract:    "health",
						Rule:        "must-have-component-field",
						Description: "HealthReport has empty Component field",
						Severity:    "error",
					})
				}
			}
		}
	}

	// 4. HealthCheck with cancelled context should return an error.
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	cancelFn()

	_, err := provider.HealthCheck(cancelCtx)
	if err == nil {
		violations = append(violations, ContractViolation{
			Contract:    "health",
			Rule:        "must-respect-context-cancellation",
			Description: "HealthCheck() with cancelled context did not return an error",
			Severity:    "warning",
		})
	}

	return violations
}
