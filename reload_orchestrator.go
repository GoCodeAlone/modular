package modular

import (
	"context"
	"fmt"
	"sort"
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

// defaultReloadTimeout is used when a module returns a non-positive ReloadTimeout.
const defaultReloadTimeout = 30 * time.Second

// ReloadOrchestrator coordinates configuration reloading across all registered
// Reloadable modules. It provides single-flight execution, circuit breaking,
// rollback on partial failure, and event emission via the observer pattern.
//
// Note: Application-level integration (Application.RequestReload(), WithDynamicReload()
// builder option) will be added when the Application interface is extended in a follow-up.
type ReloadOrchestrator struct {
	mu          sync.RWMutex
	reloadables map[string]Reloadable

	requestCh chan ReloadRequest
	stopped   atomic.Bool
	stopOnce  sync.Once

	processing atomic.Bool

	// Circuit breaker state
	cbMu        sync.Mutex
	failures    int
	lastFailure time.Time
	circuitOpen bool

	logger  Logger
	subject Subject
}

// nopLogger is a no-op Logger used when nil is passed.
type nopLogger struct{}

func (nopLogger) Info(_ string, _ ...any)  {}
func (nopLogger) Error(_ string, _ ...any) {}
func (nopLogger) Warn(_ string, _ ...any)  {}
func (nopLogger) Debug(_ string, _ ...any) {}

// NewReloadOrchestrator creates a new ReloadOrchestrator with the given logger and event subject.
// If logger is nil, a no-op logger is used.
func NewReloadOrchestrator(logger Logger, subject Subject) *ReloadOrchestrator {
	if logger == nil {
		logger = nopLogger{}
	}
	return &ReloadOrchestrator{
		reloadables: make(map[string]Reloadable),
		requestCh:   make(chan ReloadRequest, 100),
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

// RequestReload enqueues a reload request. It returns an error if the orchestrator
// is stopped, the request channel is full, or the circuit breaker is open.
//
// The method is safe to call concurrently with Stop(). A recover guard protects
// against the send-on-closed-channel panic that can occur when Stop() closes
// requestCh between the stopped check and the channel send.
func (o *ReloadOrchestrator) RequestReload(ctx context.Context, trigger ReloadTrigger, diff ConfigDiff) (retErr error) {
	if o.stopped.Load() {
		return ErrReloadStopped
	}
	if o.isCircuitOpen() {
		return ErrReloadCircuitBreakerOpen
	}

	// Recover from a send on closed channel if Stop() races between the
	// stopped check above and the channel send below.
	defer func() {
		if r := recover(); r != nil {
			retErr = ErrReloadStopped
		}
	}()

	select {
	case o.requestCh <- ReloadRequest{Trigger: trigger, Diff: diff, Ctx: ctx}:
		return nil
	default:
		return ErrReloadChannelFull
	}
}

// Start begins the background goroutine that drains the reload request queue.
func (o *ReloadOrchestrator) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				o.logger.Error("panic recovered in reload orchestrator loop", "error", r)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-o.requestCh:
				if !ok {
					return
				}
				o.handleReload(ctx, req)
			}
		}
	}()
}

// handleReload derives a properly scoped context for a single reload request and
// processes it. The context is cancelled immediately after processReload returns
// to avoid resource leaks from accumulated timers in the processing loop.
//
// The reload context is rooted in parentCtx (the Start context) so that stopping
// the orchestrator always cancels in-flight work. When the request carries its
// own context, both its deadline and cancellation are wired in: deadline via
// context.WithDeadline, and cancellation via a background goroutine that watches
// req.Ctx.Done(). This ensures callers who cancel req.Ctx abort the reload.
func (o *ReloadOrchestrator) handleReload(parentCtx context.Context, req ReloadRequest) {
	rctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if req.Ctx != nil {
		// Apply deadline if present.
		if deadline, ok := req.Ctx.Deadline(); ok {
			rctx, cancel = context.WithDeadline(rctx, deadline) //nolint:contextcheck // deadline from request
			defer cancel()
		}

		// Propagate cancellation from the request context. When req.Ctx is
		// cancelled, cancel rctx so module Reload calls see cancellation.
		go func() {
			defer func() {
				if r := recover(); r != nil {
					o.logger.Error("panic recovered in reload cancellation propagator", "error", r)
				}
			}()
			select {
			case <-req.Ctx.Done():
				cancel()
			case <-rctx.Done():
				// rctx already done (parent cancelled or reload finished); stop goroutine.
			}
		}()
	}

	if err := o.processReload(rctx, req); err != nil {
		o.logger.Error("Reload failed", "trigger", req.Trigger.String(), "error", err)
	}
}

// Stop signals the background goroutine to exit. It is safe to call multiple times.
func (o *ReloadOrchestrator) Stop() {
	o.stopOnce.Do(func() {
		o.stopped.Store(true)
		close(o.requestCh)
	})
}

// processReload executes a single reload request with atomic single-flight semantics,
// rollback on partial failure, and event emission.
func (o *ReloadOrchestrator) processReload(ctx context.Context, req ReloadRequest) error {
	// Single-flight: only one reload at a time.
	if !o.processing.CompareAndSwap(false, true) {
		o.logger.Warn("Reload already in progress, skipping request")
		return ErrReloadInProgress
	}
	defer o.processing.Store(false)

	// Noop if no changes — emit noop without a misleading "started" event.
	if !req.Diff.HasChanges() {
		o.emitEvent(ctx, EventTypeConfigReloadNoop, map[string]any{
			"trigger": req.Trigger.String(),
			"diffId":  req.Diff.DiffID,
		})
		return nil
	}

	// Emit started event only when there are actual changes to apply.
	o.emitEvent(ctx, EventTypeConfigReloadStarted, map[string]any{
		"trigger": req.Trigger.String(),
		"diffId":  req.Diff.DiffID,
		"summary": req.Diff.ChangeSummary(),
	})

	// Build the list of changes for the Reloadable interface.
	changes := o.buildChanges(req.Diff)

	// Snapshot current reloadables under read lock, sorted by name for
	// deterministic reload/rollback ordering.
	o.mu.RLock()
	targets := make([]reloadEntry, 0, len(o.reloadables))
	for name, mod := range o.reloadables {
		targets = append(targets, reloadEntry{name: name, module: mod})
	}
	o.mu.RUnlock()

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].name < targets[j].name
	})

	// Track which modules have been successfully reloaded (for rollback).
	var applied []reloadEntry

	for _, t := range targets {
		if !t.module.CanReload() {
			o.logger.Info("Module cannot reload, skipping", "module", t.name)
			continue
		}

		timeout := t.module.ReloadTimeout()
		if timeout <= 0 {
			timeout = defaultReloadTimeout
		}
		rctx, cancel := context.WithTimeout(ctx, timeout)

		err := t.module.Reload(rctx, changes)
		cancel()

		if err != nil {
			o.logger.Error("Module reload failed, initiating rollback",
				"module", t.name, "error", err)

			// Rollback already-applied modules in reverse order.
			o.rollback(ctx, applied, changes)

			o.recordFailure()
			o.emitEvent(ctx, EventTypeConfigReloadFailed, map[string]any{
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
	o.emitEvent(ctx, EventTypeConfigReloadCompleted, map[string]any{
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
		if timeout <= 0 {
			timeout = defaultReloadTimeout
		}
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
func (o *ReloadOrchestrator) emitEvent(ctx context.Context, eventType string, data map[string]any) {
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
