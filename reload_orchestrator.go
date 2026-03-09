package modular

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ReloadRequest represents a pending configuration reload request.
type ReloadRequest struct {
	Trigger ReloadTrigger
	Diff    ConfigDiff
	Ctx     context.Context
}

// reloadEntry pairs a module name with its Reloadable implementation.
type reloadEntry struct {
	name   string
	module Reloadable
}

// ReloadOrchestrator coordinates configuration reloading across all registered
// Reloadable modules. It provides single-flight execution, circuit breaking,
// rollback on partial failure, and event emission via the observer pattern.
type ReloadOrchestrator struct {
	mu          sync.RWMutex
	reloadables map[string]Reloadable

	requestCh chan ReloadRequest
	stopCh    chan struct{}

	processing atomic.Bool

	// Circuit breaker state
	cbMu        sync.Mutex
	failures    int
	lastFailure time.Time
	circuitOpen bool

	logger  Logger
	subject Subject
}

// NewReloadOrchestrator creates a new ReloadOrchestrator with the given logger and event subject.
func NewReloadOrchestrator(logger Logger, subject Subject) *ReloadOrchestrator {
	return &ReloadOrchestrator{
		reloadables: make(map[string]Reloadable),
		requestCh:   make(chan ReloadRequest, 100),
		stopCh:      make(chan struct{}),
		logger:      logger,
		subject:     subject,
	}
}

// RegisterReloadable registers a named module as reloadable.
func (o *ReloadOrchestrator) RegisterReloadable(name string, module Reloadable) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reloadables[name] = module
}

// RequestReload enqueues a reload request. It returns an error if the request
// channel is full or the circuit breaker is open.
func (o *ReloadOrchestrator) RequestReload(ctx context.Context, trigger ReloadTrigger, diff ConfigDiff) error {
	if o.isCircuitOpen() {
		return errors.New("reload circuit breaker is open; backing off")
	}
	select {
	case o.requestCh <- ReloadRequest{Trigger: trigger, Diff: diff, Ctx: ctx}:
		return nil
	default:
		return errors.New("reload request channel is full")
	}
}

// Start begins the background goroutine that drains the reload request queue.
func (o *ReloadOrchestrator) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-o.stopCh:
				return
			case req, ok := <-o.requestCh:
				if !ok {
					return
				}
				// Use the request's context if provided, otherwise use the start context.
				rctx := req.Ctx
				if rctx == nil {
					rctx = ctx
				}
				if err := o.processReload(rctx, req); err != nil {
					o.logger.Error("Reload failed", "trigger", req.Trigger.String(), "error", err)
				}
			}
		}
	}()
}

// Stop signals the background goroutine to exit and closes the request channel.
func (o *ReloadOrchestrator) Stop() {
	close(o.stopCh)
}

// processReload executes a single reload request with atomic single-flight semantics,
// rollback on partial failure, and event emission.
func (o *ReloadOrchestrator) processReload(ctx context.Context, req ReloadRequest) error {
	// Single-flight: only one reload at a time.
	if !o.processing.CompareAndSwap(false, true) {
		o.logger.Warn("Reload already in progress, skipping request")
		return errors.New("reload already in progress")
	}
	defer o.processing.Store(false)

	// Emit started event.
	o.emitEvent(ctx, EventTypeConfigReloadStarted, map[string]interface{}{
		"trigger": req.Trigger.String(),
		"diffId":  req.Diff.DiffID,
		"summary": req.Diff.ChangeSummary(),
	})

	// Noop if no changes.
	if !req.Diff.HasChanges() {
		o.emitEvent(ctx, EventTypeConfigReloadNoop, map[string]interface{}{
			"trigger": req.Trigger.String(),
			"diffId":  req.Diff.DiffID,
		})
		return nil
	}

	// Build the list of changes for the Reloadable interface.
	changes := o.buildChanges(req.Diff)

	// Snapshot current reloadables under read lock.
	o.mu.RLock()
	var targets []reloadEntry
	for name, mod := range o.reloadables {
		targets = append(targets, reloadEntry{name: name, module: mod})
	}
	o.mu.RUnlock()

	// Track which modules have been successfully reloaded (for rollback).
	var applied []reloadEntry

	for _, t := range targets {
		if !t.module.CanReload() {
			o.logger.Info("Module cannot reload, skipping", "module", t.name)
			continue
		}

		timeout := t.module.ReloadTimeout()
		rctx, cancel := context.WithTimeout(ctx, timeout)

		err := t.module.Reload(rctx, changes)
		cancel()

		if err != nil {
			o.logger.Error("Module reload failed, initiating rollback",
				"module", t.name, "error", err)

			// Rollback already-applied modules in reverse order.
			o.rollback(ctx, applied, changes)

			o.recordFailure()
			o.emitEvent(ctx, EventTypeConfigReloadFailed, map[string]interface{}{
				"trigger":      req.Trigger.String(),
				"diffId":       req.Diff.DiffID,
				"failedModule": t.name,
				"error":        err.Error(),
			})
			return fmt.Errorf("reload failed at module %s: %w", t.name, err)
		}

		applied = append(applied, t)
	}

	o.recordSuccess()
	o.emitEvent(ctx, EventTypeConfigReloadCompleted, map[string]interface{}{
		"trigger":       req.Trigger.String(),
		"diffId":        req.Diff.DiffID,
		"modulesLoaded": len(applied),
	})
	return nil
}

// buildChanges converts a ConfigDiff into a flat slice of ConfigChange entries.
func (o *ReloadOrchestrator) buildChanges(diff ConfigDiff) []ConfigChange {
	var changes []ConfigChange
	for path, fc := range diff.Added {
		changes = append(changes, ConfigChange{
			FieldPath: path,
			OldValue:  fmt.Sprintf("%v", fc.OldValue),
			NewValue:  fmt.Sprintf("%v", fc.NewValue),
			Source:    "diff",
		})
	}
	for path, fc := range diff.Changed {
		changes = append(changes, ConfigChange{
			FieldPath: path,
			OldValue:  fmt.Sprintf("%v", fc.OldValue),
			NewValue:  fmt.Sprintf("%v", fc.NewValue),
			Source:    "diff",
		})
	}
	for path, fc := range diff.Removed {
		changes = append(changes, ConfigChange{
			FieldPath: path,
			OldValue:  fmt.Sprintf("%v", fc.OldValue),
			NewValue:  fmt.Sprintf("%v", fc.NewValue),
			Source:    "diff",
		})
	}
	return changes
}

// rollback attempts to reverse already-applied changes on modules in reverse order.
// This is best-effort: errors are logged but not propagated.
func (o *ReloadOrchestrator) rollback(ctx context.Context, applied []reloadEntry, originalChanges []ConfigChange) {
	// Build reverse changes (swap old and new values).
	reverseChanges := make([]ConfigChange, len(originalChanges))
	for i, c := range originalChanges {
		reverseChanges[i] = ConfigChange{
			Section:   c.Section,
			FieldPath: c.FieldPath,
			OldValue:  c.NewValue,
			NewValue:  c.OldValue,
			Source:    "rollback",
		}
	}

	// Apply in reverse order.
	for i := len(applied) - 1; i >= 0; i-- {
		t := applied[i]
		timeout := t.module.ReloadTimeout()
		rctx, cancel := context.WithTimeout(ctx, timeout)

		if err := t.module.Reload(rctx, reverseChanges); err != nil {
			o.logger.Error("Rollback failed for module", "module", t.name, "error", err)
		} else {
			o.logger.Info("Rollback succeeded for module", "module", t.name)
		}
		cancel()
	}
}

// emitEvent sends a CloudEvent via the configured subject.
func (o *ReloadOrchestrator) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	if o.subject == nil {
		return
	}
	event := NewCloudEvent(eventType, "modular.reload.orchestrator", data, nil)
	if err := o.subject.NotifyObservers(ctx, event); err != nil {
		o.logger.Debug("Failed to emit reload event", "eventType", eventType, "error", err)
	}
}

// Circuit breaker methods.

const (
	circuitBreakerThreshold = 3
	circuitBreakerBaseDelay = 2 * time.Second
	circuitBreakerMaxDelay  = 2 * time.Minute
)

func (o *ReloadOrchestrator) isCircuitOpen() bool {
	o.cbMu.Lock()
	defer o.cbMu.Unlock()
	if !o.circuitOpen {
		return false
	}
	// Check if the backoff period has elapsed.
	if time.Since(o.lastFailure) > o.backoffDuration() {
		o.circuitOpen = false
		o.logger.Info("Reload circuit breaker reset after backoff")
		return false
	}
	return true
}

func (o *ReloadOrchestrator) recordSuccess() {
	o.cbMu.Lock()
	defer o.cbMu.Unlock()
	o.failures = 0
	o.circuitOpen = false
}

func (o *ReloadOrchestrator) recordFailure() {
	o.cbMu.Lock()
	defer o.cbMu.Unlock()
	o.failures++
	o.lastFailure = time.Now()
	if o.failures >= circuitBreakerThreshold {
		o.circuitOpen = true
		o.logger.Warn("Reload circuit breaker opened",
			"failures", o.failures,
			"backoff", o.backoffDuration().String())
	}
}

func (o *ReloadOrchestrator) backoffDuration() time.Duration {
	if o.failures <= 0 {
		return circuitBreakerBaseDelay
	}
	d := circuitBreakerBaseDelay
	for i := 1; i < o.failures; i++ {
		d *= 2
		if d > circuitBreakerMaxDelay {
			return circuitBreakerMaxDelay
		}
	}
	return d
}
