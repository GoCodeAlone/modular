# Design Brief: FR-045 Dynamic Configuration Reload

Status: Draft
Owner: TBD
Date: 2025-09-07

## 1. Problem / Goal
Allow safe, bounded-latency hot reload of explicitly tagged configuration fields without full process restart. Non-dynamic fields continue to require restart, preserving determinism.

## 2. Scope
In Scope:
- Field-level opt-in via struct tag: `dynamic:"true"` (boolean presence)
- Module opt-in interface: `type Reloadable interface { Reload(ctx context.Context, changed []ConfigChange) error }`
- Change detection across feeders (env/file/programmatic) with provenance awareness
- Atomic validation (all changed dynamic fields validated together before commit)
- Event emission (CloudEvents + internal observer) for: reload.start, reload.success, reload.failed, reload.noop
- Backoff & jitter for repeated failures of same field set
- Guardrails: max concurrent reload operations = 1 (queued), max frequency default 1 per 5s per module

Out of Scope (Future):
- Partial rollback mid-execution (failure aborts whole batch)
- Schema evolution (adding/removing fields at runtime)
- Dynamic enablement of modules

## 3. Key Concepts
ConfigSnapshot: immutable view of active config
PendingSnapshot: candidate snapshot under validation
ConfigChange: { Section, FieldPath, OldValue(any), NewValue(any), Source(feederID) }
ReloadPlan: grouping of changes by module + affected services

## 4. Flow
1. Trigger Sources:
   - File watcher (yaml/json/toml) debounce 250ms
   - Explicit API: Application.RequestReload(sectionNames ...string)
2. Diff current vs newly loaded raw config
3. Filter to fields tagged dynamic
4. If none → emit reload.noop
5. Build candidate struct(s); apply defaults; run validation (including custom validators)
6. If validation fails → emit reload.failed (with reasons, redacted); backoff
7. For each module implementing Reloadable with at least one affected field:
   - Invoke Reload(ctx, changedSubset) sequentially (ordered by registration)
   - Collect errors; on first error mark failure → emit reload.failed; do not commit snapshot
8. If all succeed → swap active snapshot atomically → emit reload.success

## 5. Data / Concurrency Model
- Single goroutine reload coordinator + channel of reload requests
- Snapshot pointer swap protected by RWMutex
- Readers acquire RLock (service resolution / module access)
- Reload obtains full Lock during commit only (short critical section)

## 6. Tag & Validation Strategy
- Use struct tag: `dynamic:"true"` on individual fields
- Nested structs allowed; dynamic status is not inherited (must be explicit)
- Reject reload if a changed field lacks dynamic tag (forces restart path)

## 7. API Additions
```go
// Reload request (internal)
type ConfigChange struct {
    Section     string
    FieldPath   string
    OldValue    any
    NewValue    any
    Source      string
}

type Reloadable interface {
    Reload(ctx context.Context, changed []ConfigChange) error
}

// Application level
func (a *StdApplication) RequestReload(sections ...string) error
```

Observer Events (names):
- config.reload.start
- config.reload.success
- config.reload.failed
- config.reload.noop

## 8. Error Handling
- Aggregate validation errors (field -> reason), wrap into ReloadError (implements error, exposes slice)
- Reloadable module failure returns error → abort pipeline
- Backoff map keyed by canonical change set hash (sorted FieldPaths + section) with exponential (base 2, cap 2m)

## 9. Metrics (to integrate with spec success criteria)
- reload_duration_ms (histogram)
- reload_changes_count
- reload_failed_total (counter, reason labels: validation|module|internal)
- reload_skipped_undynamic_total
- reload_inflight (gauge 0/1)

## 10. Security / Secrets
- Redact values in events/logs if field classified secret (reuse secret classification model planned FR-049)

## 11. Edge Cases
- Concurrent identical reload requests collapse into one execution
- Validation passes but module reload fails → no commit
- File partially written (temporary invalid syntax) → parse error → ignored with logged warning & retry
- Rapid thrash (config flapping) → debounced; last stable snapshot wins

## 12. Testing Strategy
Unit:
- Diff computation (single, nested, list-based fields)
- Dynamic tag enforcement rejections
- Validation aggregation
- Backoff growth & cap
Integration:
- Two modules, one dynamic field each; change triggers sequential Reload calls
- Mixed dynamic & non-dynamic changes: only dynamic applied
- Failure in second module aborts snapshot commit
- Secret field change emits redacted event payload
Race / Concurrency:
- Repeated RequestReload while long-running module reload executes (queue & ordering)

BDD Acceptance Mapping:
- Matches FR-045 scenarios in main spec acceptance plan.

## 13. Migration / Backward Compatibility
- No breaking change; dynamic tags additive
- Modules may adopt Reloadable gradually

## 14. Open Questions (to confirm before implementation)
1. Should non-dynamic changes optionally emit advisory event? (default yes, suppressed w/ option) 
2. Provide global opt-out of file watcher? (likely yes via builder option)

## 15. Implementation Phases
Phase 1: Core diff + tag recognition + RequestReload API + events (no file watcher)
Phase 2: File watcher + debounce
Phase 3: Metrics + backoff + redaction integration
Phase 4: Documentation & examples
