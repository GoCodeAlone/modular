# Phase 1 Data Model

This document captures conceptual types introduced or formalized by planned enhancements. It segregates CORE vs MODULE scope to enforce architectural boundaries per constitution Articles XII & XVI.

## CORE Enumerations
- ServiceScope: Global | Tenant | Instance (default Global)
- HealthStatus: Healthy | Degraded | Unhealthy
- TenantGuardMode: Strict | Permissive
- BackfillStrategy: None | All | Last | Bounded | TimeWindow (formalizing existing variants)
- ErrorCategory: ConfigError | ValidationError | DependencyError | LifecycleError | SecurityError (extensible)

## CORE Interfaces (Additive)
```go
// Reloadable implemented by modules needing dynamic config application.
type Reloadable interface {
	// Reload applies validated diff. Must be idempotent and fast.
	Reload(ctx context.Context, diff ConfigDiff) error
}

// HealthReporter implemented by modules to expose health status.
type HealthReporter interface {
	HealthReport(ctx context.Context) HealthResult
}
```

## CORE Struct Concepts
- ConfigDiff: changedFields map[path]FieldChange; timestamp
- FieldChange: Old any; New any
- HealthResult: Status HealthStatus; Message string; Timestamp time.Time; Optional bool
- AggregateHealthSnapshot: OverallStatus HealthStatus; ReadinessStatus HealthStatus; ModuleResults []HealthResult; GeneratedAt time.Time
- SecretValue: opaque wrapper ensuring redacted String() / fmt output

## MODULE Scope (Auth, Scheduler, ACME)
No new domain entities moved into CORE. Scheduler backfill policy object remains in scheduler module (exposed config struct only). Auth OIDC provider SPI defined inside auth module (not exported by core root). ACME escalation event schema lives in letsencrypt module.

## Relationships
- Application → ServiceRegistry (1:1)
- ServiceRegistryEntry → ServiceScope (1:1)
- Application → Modules[*Module]
- Module (optional) → Reloadable
- Module (optional) → HealthReporter
- AggregateHealthService → Modules (poll HealthReporter)

## Validation Rules
- ServiceScope must be valid enum; default Global.
- Dynamic reload: attempt to change non-dynamic field → validation error; diff aborted.
- HealthResult.Timestamp not > now + 2s tolerance.
- SecretValue always redacts; Reveal() only in controlled internal paths (never logs).

## Reload State Transition
1. Baseline snapshot current config
2. Re-collect config via feeders
3. Validate full candidate config
4. Derive diff restricted to dynamic-tagged fields
5. If diff empty → emit ConfigReloadNoop event
6. Sequentially invoke Reloadable modules (original start order) each under timeout
7. Emit ConfigReloadCompleted (success/failure)

## Health Aggregation Algorithm (Summary)
Collect reports with per-module timeout; compute worst OverallStatus; compute ReadinessStatus ignoring Optional failures unless Unhealthy. Cache snapshot atomically; emit HealthEvaluated event.

## Performance Considerations
- Diff generation: O(changed_dynamic_fields)
- Health snapshot: O(reporters) per interval (target ≤5ms typical)
- No added locks on service lookup path.

## Extensibility Points
- Append ErrorCategory constants (non-breaking)
- Future secret classification levels (reserved field in SecretValue)
- Additional builder options can register new HealthReporter sources without modifying aggregator interface.

## Scope Enforcement Note
No MODULE-specific data structures appear in CORE sections above. Any attempt to add auth provider structs or scheduler internal job state types into core will raise: "Scope violation in design artifact" during future checks.

