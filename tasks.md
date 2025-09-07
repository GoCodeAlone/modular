# Tasks: Baseline Specification Enablement (Dynamic Reload & Health Aggregation + Enhancements)

**Input**: Design artifacts in `specs/001-baseline-specification-for`
**Prerequisites**: plan.md, research.md, data-model.md, contracts/, quickstart.md

## Execution Flow (applied)
1. Loaded plan.md & extracted builder options / observer events.
2. Parsed data-model entities & enums (ServiceScope, HealthStatus, etc.).
3. Parsed contracts (`health.md`, `reload.md`) → generated contract test tasks.
4. Derived tasks (tests first) for each enhancement & pattern evolution.
5. Added integration tests for representative user stories (startup, failure rollback, multi-tenancy, graceful shutdown, config provenance, ambiguous service tie-break, scheduler catch-up, ACME escalation, reload, health aggregation, secret redaction).
6. Ordered tasks to enforce RED → GREEN.
7. Added dependency graph & parallel groups.

Legend:
- `[CORE]` Root framework (no writes under `modules/`)
- `[MODULE:<name>]` Specific module scope only
- `[P]` Parallel-capable (separate files / no dependency)

## Phase 3.1 Setup & Baseline
T001 [CORE] Create baseline benchmarks `internal/benchmark/benchmark_baseline_test.go` (bootstrap & lookup)  

## Phase 3.2 Contract & Feature Tests (RED)
T002 [CORE][P] Contract test (reload no-op) `internal/reload/reload_noop_test.go` referencing `contracts/reload.md`
T003 [CORE][P] Contract test (reload dynamic apply) `internal/reload/reload_dynamic_apply_test.go`
T004 [CORE][P] Contract test (reload reject static) `internal/reload/reload_reject_static_change_test.go`
T005 [CORE][P] Contract test (health readiness excludes optional) `internal/health/health_readiness_optional_test.go` referencing `contracts/health.md`
T006 [CORE][P] Contract test (health precedence) `internal/health/health_precedence_test.go`
T007 [CORE][P] Service scope listing test `internal/registry/service_scope_listing_test.go`
T008 [CORE][P] Tenant guard strict vs permissive test `internal/tenant/tenant_guard_mode_test.go`
T009 [CORE][P] Decorator ordering & tie-break test `internal/decorator/decorator_order_tiebreak_test.go`
T010 [CORE][P] Tie-break ambiguity error test `internal/registry/service_tiebreak_ambiguity_test.go`
T011 [CORE][P] Isolation leakage prevention test `internal/tenant/tenant_isolation_leak_test.go`
T012 [CORE][P] Reload race safety test `internal/reload/reload_race_safety_test.go`
T013 [CORE][P] Health interval & jitter test `internal/health/health_interval_jitter_test.go`
T014 [CORE][P] Metrics emission test (reload & health) `internal/platform/metrics/metrics_reload_health_emit_test.go`
T015 [CORE][P] Error taxonomy classification test `internal/errors/error_taxonomy_classification_test.go`
T016 [CORE][P] Secret redaction logging test `internal/secrets/secret_redaction_log_test.go`
T017 [CORE][P] Secret provenance redaction test `internal/secrets/secret_provenance_redaction_test.go`
T018 [CORE][P] Scheduler catch-up bounded policy test `modules/scheduler/scheduler_catchup_policy_test.go`
T019 [MODULE:letsencrypt][P] ACME escalation event test `modules/letsencrypt/acme_escalation_event_test.go`
T020 [MODULE:auth][P] OIDC SPI multi-provider test `modules/auth/oidc_spi_multi_provider_test.go`
T021 [MODULE:auth][P] Auth multi-mechanisms coexist test `modules/auth/auth_multi_mechanisms_coexist_test.go`
T022 [MODULE:auth][P] OIDC error taxonomy mapping test `modules/auth/auth_oidc_error_taxonomy_test.go`

## Phase 3.2 Integration Scenario Tests (User Stories) (RED)
T023 [CORE][P] Integration: startup dependency resolution `integration/startup_order_test.go`
T024 [CORE][P] Integration: failure rollback & reverse stop `integration/failure_rollback_test.go`
T025 [CORE][P] Integration: multi-tenancy isolation under load `integration/tenant_isolation_load_test.go`
T026 [CORE][P] Integration: config provenance & required field failure reporting `integration/config_provenance_error_test.go`
T027 [CORE][P] Integration: graceful shutdown ordering `integration/graceful_shutdown_order_test.go`
T028 [CORE][P] Integration: scheduler downtime catch-up bounding `integration/scheduler_catchup_integration_test.go`
T029 [CORE][P] Integration: dynamic reload + health interplay `integration/reload_health_interplay_test.go`
T030 [CORE][P] Integration: secret leakage scan `integration/secret_leak_scan_test.go`

## Phase 3.3 Core Implementations (GREEN)
T031 [CORE] Implement `ServiceScope` enum & registry changes `internal/registry/service_registry.go`
T032 [CORE] Implement tenant guard mode + builder `WithTenantGuardMode()` `internal/tenant/tenant_guard.go`
T033 [CORE] Implement decorator priority metadata & tie-break `internal/decorator/decorator_chain.go`
T034 [CORE] Implement dynamic reload pipeline + builder `WithDynamicReload()` `internal/reload/pipeline.go`
T035 [CORE] Implement ConfigReload events `internal/reload/events.go`
T036 [CORE] Implement health aggregator + builder `WithHealthAggregator()` `internal/health/aggregator.go`
T037 [CORE] Emit HealthEvaluated event `internal/health/events.go`
T038 [CORE] Implement error taxonomy helpers `errors_taxonomy.go`
T039 [CORE] Implement SecretValue wrapper & logging integration `internal/secrets/secret_value.go`
T040 [CORE] Implement scheduler catch-up policy integration point `internal/scheduler/policy_bridge.go`
T041 [MODULE:scheduler] Implement bounded catch-up policy logic `modules/scheduler/policy.go`
T042 [MODULE:letsencrypt] Implement escalation event emission `modules/letsencrypt/escalation.go`
T043 [MODULE:auth] Implement OIDC Provider SPI & builder option `modules/auth/oidc_provider.go`
T044 [MODULE:auth] Integrate taxonomy helpers in SPI errors `modules/auth/oidc_errors.go`
T045 [CORE] Implement tie-break diagnostics enhancements `internal/registry/service_resolution.go`
T046 [CORE] Implement isolation/leakage guard path `internal/tenant/tenant_isolation.go`
T047 [CORE] Add reload concurrency safety (mutex/atomic snapshot) `internal/reload/safety.go`
T048 [CORE] Implement health ticker & jitter `internal/health/ticker.go`
T049 [CORE] Implement metrics counters & histograms `internal/platform/metrics/reload_health_metrics.go`
T050 [CORE] Apply secret redaction in provenance tracker `internal/config/provenance_redaction.go`

## Phase 3.4 Integration & Cross-Cutting
T051 [CORE] Wire metrics + events into application builder `application.go`
T052 [CORE] Update examples with dynamic reload & health usage `examples/dynamic-health/main.go`

## Phase 3.5 Hardening & Benchmarks
T053 [CORE] Post-change benchmarks `internal/benchmark/benchmark_postchange_test.go`
T054 [CORE] Reload latency & health aggregation benchmarks `internal/benchmark/benchmark_reload_health_test.go`

## Phase 3.6 Test Finalization (Quality Gate)
Purpose: Enforce template Phase 3.6 requirements (no placeholders, full assertions, deterministic timing, schema & API stability) prior to final validation.

T060 [CORE] Placeholder & skip scan remediation script `scripts/test_placeholder_scan.sh` (fails if any `TODO|FIXME|t.Skip|placeholder|future implementation` remains in `*_test.go`)
T061 [CORE] Coverage gap critical path additions `internal/test/coverage_gap_test.go` (adds assertions for uncovered error branches & boundary conditions revealed by coverage run)
T062 [CORE] Timing determinism audit `internal/test/timing_audit_test.go` (fails if tests rely on arbitrary `time.Sleep` >50ms without `//deterministic-ok` annotation)
T063 [CORE] Event schema snapshot guard `internal/observer/event_schema_snapshot_test.go` (captures JSON schema of emitted lifecycle/health/reload events; diff required for changes)
T064 [CORE] Builder option & observer event doc parity test `internal/builder/options_doc_parity_test.go` (verifies every `With*` option & event type has matching section in `DOCUMENTATION.md` / relevant module README)
T065 [CORE] Public API diff & interface widening guard `internal/api/api_diff_test.go` (compares exported symbols against baseline snapshot under `internal/api/.snapshots`)

## Phase 3.7 Documentation & Polish
T055 [CORE][P] Update `DOCUMENTATION.md` (reload, health, taxonomy, secrets)
T056 [MODULE:auth][P] Update `modules/auth/README.md` (OIDC SPI, error taxonomy)
T057 [MODULE:letsencrypt][P] Update `modules/letsencrypt/README.md` (escalation events)
T058 [MODULE:scheduler][P] Update `modules/scheduler/README.md` (catch-up policies)
T059 [CORE][P] Add dedicated docs `docs/errors_secrets.md`

## Phase 3.8 Final Validation
T066 [CORE] Final validation script & update spec/plan statuses `scripts/validate-feature.sh`

## Wave Overview
Wave 0: Baseline scaffolding  
Wave 1: All RED tests (contracts + integration)  
Wave 2: Core feature implementations (ServiceScope, reload, health, decorators, tenant guards, error taxonomy, secrets)  
Wave 3: Module-specific implementations (auth OIDC, scheduler policy, letsencrypt escalation)  
Wave 4: Cross-cutting integration (metrics, events, application wiring)  
Wave 5: Test Finalization (T060–T065)  
Wave 6: Final Validation (T066)

## Parallel Execution Guidance
RED test wave (independent): T002–T022, T023–T030 may run concurrently (distinct files).  
GREEN implementation wave: T031–T050 follow respective test dependencies (see graph).  
Docs & polish tasks (T055–T059) run parallel after core implementations green.

## Dependency Graph (Abbrev)
T031←T007; T032←T008; T033←T009; T034←(T002,T003,T004); T035←T034; T036←(T005,T006); T037←T036; T038←T015; T039←T016; T040←T018; T041←T018; T042←T019; T043←T020; T044←(T022,T038); T045←(T010,T031); T046←T011; T047←T012; T048←T013; T049←(T014,T034,T036); T050←(T016,T039); T051←(T035,T037,T049); T052←(T034,T036); T053←(T051); T054←(T034,T036,T049); T055–T059←(T031..T052); T060–T065←(T055–T059, T001–T054); T066←ALL.

## Classification Summary
| Category | Count |
|----------|-------|
| CORE | 44 |
| MODULE:auth | 6 |
| MODULE:scheduler | 2 |
| MODULE:letsencrypt | 3 |
| TOTAL | 55 |

## Validation
- All functionalities classified (no unclassified items).  
- No mis-scoped tasks (CORE tasks stay outside `modules/`; MODULE tasks confined).  
- Pattern-first: every implementation task has preceding RED test.  
- Builder options introduced only via additive options (dynamic reload, health aggregator, tenant guard, OIDC provider, catch-up policy).  
- Observer events have test + implementation (ConfigReload*, HealthEvaluated, CertificateRenewalEscalated).  
- No interface widening; only new interfaces (`Reloadable`, `HealthReporter`).

## Notes
- Failing tests may initially use build tag `//go:build planned` to keep baseline green until implementation phase starts.
- Benchmarks optional but recommended for regression tracking; remove tag once stable.
- Integration tests avoid external network where possible; mock ACME interactions via local test harness.
- Test Finalization phase enforces zero tolerance for lingering placeholders & undocumented public surface changes before final validation.