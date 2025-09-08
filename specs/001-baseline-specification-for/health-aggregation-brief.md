# Design Brief: FR-048 Aggregate Health & Readiness

Status: Draft
Owner: TBD
Date: 2025-09-07

## 1. Problem / Goal
Provide a standardized way for modules to expose granular health/readiness signals and aggregate them into a single consumable endpoint / API with correct treatment of optional vs required modules.

## 2. Scope
In Scope:
- Module-level interface for health declarations
- Distinct concepts: Readiness (can accept traffic) vs Health (ongoing quality)
- Status tri-state: healthy | degraded | unhealthy
- Aggregation policy: readiness ignores optional module failures; health reflects worst status
- Optional HTTP handler wiring (disabled by default) returning JSON
- Event emission on state transitions with previous->current
- Caching layer (default TTL 250ms) to avoid hot path thrash

Out of Scope (Phase 1):
- Per-check latency metrics (added later)
- Structured remediation suggestions
- Push model (modules pushing state changes) – initial design is pull on interval

## 3. Interfaces
```go
type HealthStatus string
const (
  StatusHealthy  HealthStatus = "healthy"
  StatusDegraded HealthStatus = "degraded"
  StatusUnhealthy HealthStatus = "unhealthy"
)

type HealthReport struct {
  Module       string        `json:"module"`
  Component    string        `json:"component,omitempty"`
  Status       HealthStatus  `json:"status"`
  Message      string        `json:"message,omitempty"`
  CheckedAt    time.Time     `json:"checkedAt"`
  ObservedSince time.Time    `json:"observedSince"`
  Optional     bool          `json:"optional"`
  Details      map[string]any `json:"details,omitempty"`
}

type HealthProvider interface {
  HealthCheck(ctx context.Context) ([]HealthReport, error)
}
```

Aggregator API:
```go
type AggregatedHealth struct {
  Readiness HealthStatus `json:"readiness"`
  Health    HealthStatus `json:"health"`
  Reports   []HealthReport `json:"reports"`
  GeneratedAt time.Time `json:"generatedAt"`
}

type HealthAggregator interface {
  Collect(ctx context.Context) (AggregatedHealth, error)
}
```

## 4. Aggregation Rules
Readiness:
- Start at healthy
- For each report where Optional=false:
  - unhealthy -> readiness=unhealthy
  - degraded (only if no unhealthy) -> readiness=degraded
Health:
- Worst of all reports (optional included) by ordering healthy < degraded < unhealthy

## 5. Module Integration
- New decorator or registration helper: `RegisterHealthProvider(moduleName string, provider HealthProvider, optional bool)`
- Application retains registry: moduleName -> []provider entries
- Aggregator iterates providers on collection tick (default 1s) with timeout per provider (default 200ms)

## 6. Caching Layer
- Last AggregatedHealth stored with timestamp
- Subsequent Collect() within TTL returns cached value
- Forced collection bypass via `Collect(context.WithValue(ctx, ForceKey, true))`

## 7. Events
- Event: health.aggregate.updated (payload: previous overall, new overall, readiness change, counts)
- Emit only when either readiness or health status value changes

## 8. HTTP Handler (Optional)
Path suggestion: `/healthz` returns JSON AggregatedHealth
Enable via builder option: `WithHealthEndpoint(path string)`
Disabled by default to keep baseline lean

## 9. Error Handling
- Provider error -> treat as unhealthy report with message, unless error implements `Temporary()` and returns degraded
- Panic in provider recovered and converted to unhealthy with message "panic: <value>"

## 10. Metrics
- health_collection_duration_ms (hist)
- health_collection_failures_total (counter)
- health_status_changes_total (counter, labels: readiness|health)
- health_reports_count (gauge)

## 11. Concurrency & Performance
- Single collection goroutine on interval; providers invoked sequentially (Phase 1)
- Future optimization: parallel with bounded worker pool
- Protect shared state with RWMutex

## 12. Security / PII
- No sensitive values logged; Details map redacted via existing classification (FR-049) once integrated

## 13. Testing Strategy
Unit:
- Aggregation rule matrix (healthy/degraded/unhealthy combinations)
- Optional module exclusion from readiness
- Caching TTL behavior & forced refresh
- Provider timeout and error classification
Integration:
- Multiple providers, readiness transitions, event emission ordering
- HTTP endpoint JSON contract & content type
Race:
- Rapid successive Collect calls hitting cache vs forced refresh

## 14. Backward Compatibility
- Additive; modules implement HealthProvider when ready

## 15. Phases
Phase 1: Core interfaces + aggregator + basic collection + caching
Phase 2: HTTP endpoint + events
Phase 3: Metrics + parallelization + classification integration

## 16. Open Questions
1. Should readiness degrade if all required are healthy but >N optional are degraded? (current: no)
2. Allow per-provider custom timeout? (likely yes via registration parameter)
