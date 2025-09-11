# Tasks: Baseline Specification Enablement (Dynamic Reload, Health Aggregation & Supporting Enhancements)

**Input**: Design documents from `/specs/001-baseline-specification-for/`
**Prerequisites**: plan.md (required), data-model.md, contracts/, quickstart.md

## Classification Summary
| Scope | Count | Description |
|-------|-------|-------------|
| CORE | 22 | Framework enhancements (lifecycle, config, health, service registry) |
| MODULE | 8 | Module-specific enhancements (auth, scheduler, letsencrypt) |
| **Total** | **30** | All functionality classified, no mis-scoped tasks |

## Phase 3.1: Setup & Prerequisites
- T001 [CORE] Verify modular framework builds and passes existing tests
- T002 [CORE][P] Add build tags for failing tests to avoid breaking main during TDD
- T003 [CORE][P] Update go.mod dependencies if needed for new functionality

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE Core

### Contract & Interface Tests
- T004 [CORE][P] Create failing test for Reloadable interface in `reloadable_test.go`
- T005 [CORE][P] Create failing test for HealthReporter interface in `health_reporter_test.go`
- T006 [CORE][P] Create failing test for AggregateHealthService in `aggregate_health_test.go`
- T007 [CORE][P] Create failing test for ConfigDiff generation in `config_diff_test.go`
- T008 [CORE][P] Create failing test for ServiceScope enum in `service_scope_test.go`

### Observer Event Tests
- T009 [CORE][P] Create failing test for ConfigReloadStarted event emission in `reload_events_test.go`
- T010 [CORE][P] Create failing test for ConfigReloadCompleted event emission in `reload_events_test.go`
- T011 [CORE][P] Create failing test for HealthEvaluated event emission in `health_events_test.go`
- T012 [MODULE:letsencrypt][P] Create failing test for CertificateRenewalEscalated event in `modules/letsencrypt/escalation_test.go`

### Builder Option Tests
- T013 [CORE][P] Create failing test for WithDynamicReload() option in `application_options_test.go`
- T014 [CORE][P] Create failing test for WithHealthAggregator() option in `application_options_test.go`
- T015 [CORE][P] Create failing test for WithTenantGuardMode() option in `tenant_options_test.go`
- T016 [CORE][P] Create failing test for WithServiceScope() option in `service_registry_test.go`
- T017 [MODULE:scheduler][P] Create failing test for WithSchedulerCatchUp() in `modules/scheduler/catchup_test.go`

### Integration Scenario Tests
- T018 [CORE][P] Create failing integration test for dynamic reload flow in `integration_reload_test.go`
- T019 [CORE][P] Create failing integration test for health aggregation in `integration_health_test.go`
- T020 [CORE][P] Create failing test for reload with validation errors in `reload_validation_test.go`
- T021 [CORE][P] Create failing test for health with optional modules in `health_optional_test.go`
- T022 [CORE][P] Create failing test for concurrent reload safety in `reload_concurrency_test.go`

## Phase 3.3: Core Implementation (Only after failing tests present)

### Core Interfaces & Types
- T023 [CORE] Implement Reloadable interface in `reloadable.go`
- T024 [CORE] Implement HealthReporter interface in `health_reporter.go`
- T025 [CORE] Implement ServiceScope enum and validation in `service_scope.go`
- T026 [CORE] Implement ConfigDiff type and generation logic in `config_diff.go`
- T027 [CORE] Implement HealthResult and AggregateHealthSnapshot types in `health_types.go`

### Core Services
- T028 [CORE] Implement AggregateHealthService in `aggregate_health_service.go`
- T029 [CORE] Implement dynamic reload orchestration in `reload_orchestrator.go`
- T030 [CORE] Implement SecretValue wrapper type in `secret_value.go`

### Builder Options Implementation
- T031 [CORE] Implement WithDynamicReload() option in `application_options.go`
- T032 [CORE] Implement WithHealthAggregator() option in `application_options.go`
- T033 [CORE] Implement WithTenantGuardMode() option in `tenant_options.go`
- T034 [CORE] Implement WithServiceScope() option in `service_registry.go`

### Observer Event Implementation
- T035 [CORE] Implement ConfigReloadStarted/Completed events in `reload_events.go`
- T036 [CORE] Implement HealthEvaluated event in `health_events.go`

### Module Enhancements
- T037 [MODULE:scheduler] Implement WithSchedulerCatchUp() in `modules/scheduler/catchup.go`
- T038 [MODULE:auth] Add OIDC provider SPI in `modules/auth/oidc_provider.go`
- T039 [MODULE:letsencrypt] Implement CertificateRenewalEscalated event in `modules/letsencrypt/escalation.go`

## Phase 3.4: Integration / Adapters

### Module Integration
- T040 [MODULE:httpserver] Make HTTPServer module implement Reloadable in `modules/httpserver/reload.go`
- T041 [MODULE:database] Make Database module implement HealthReporter in `modules/database/health.go`
- T042 [MODULE:cache] Make Cache module implement HealthReporter in `modules/cache/health.go`
- T043 [MODULE:eventbus] Make EventBus module implement HealthReporter in `modules/eventbus/health.go`

### Configuration Integration
- T044 [CORE] Add dynamic field tag parsing to config validation in `config_validation.go`
- T045 [CORE] Integrate reload trigger with application lifecycle in `application.go`
- T046 [CORE] Add Health() accessor method to Application interface in `application.go`

## Phase 3.5: Hardening & Polish

### Performance & Edge Cases
- T047 [CORE][P] Add benchmarks for config diff generation in `config_diff_bench_test.go`
- T048 [CORE][P] Add benchmarks for health aggregation in `health_bench_test.go`
- T049 [CORE][P] Add timeout handling for slow HealthReporter modules in `aggregate_health_service.go`
- T050 [CORE][P] Add circuit breaker for repeated reload failures in `reload_orchestrator.go`

### Documentation & Examples
- T051 [CORE][P] Update CLAUDE.md with dynamic reload and health aggregation guidance
- T052 [CORE][P] Create example application demonstrating reload in `examples/dynamic-reload/`
- T053 [CORE][P] Create example application demonstrating health aggregation in `examples/health-monitoring/`
- T054 [CORE][P] Generate updated sample configs with dynamic tags in `configs/`

## Phase 3.6: Test Finalization

- T055 [CORE] Remove all test build tags and ensure all tests pass
- T056 [CORE] Verify TDD commit history shows RED → GREEN → REFACTOR pattern
- T057 [CORE][P] Scan for and remove any TODO/FIXME/placeholder markers in tests
- T058 [CORE][P] Verify code coverage meets thresholds for critical paths

## Dependencies

### Critical Path
1. Setup (T001-T003) must complete first
2. All Tests (T004-T022) must be written and failing before implementation
3. Core Implementation (T023-T039) can begin only after tests exist
4. Integration (T040-T046) depends on core implementation
5. Hardening (T047-T054) after functional implementation
6. Test Finalization (T055-T058) is the final gate

### Parallel Execution Examples

**Batch 1 - Initial Tests (can run together):**
```bash
Task agent T004 "Create failing Reloadable interface test" &
Task agent T005 "Create failing HealthReporter interface test" &
Task agent T006 "Create failing AggregateHealthService test" &
Task agent T007 "Create failing ConfigDiff generation test" &
Task agent T008 "Create failing ServiceScope enum test" &
wait
```

**Batch 2 - Event & Option Tests (after Batch 1):**
```bash
Task agent T009 "Create failing ConfigReloadStarted event test" &
Task agent T010 "Create failing ConfigReloadCompleted event test" &
Task agent T011 "Create failing HealthEvaluated event test" &
Task agent T013 "Create failing WithDynamicReload option test" &
Task agent T014 "Create failing WithHealthAggregator option test" &
wait
```

**Batch 3 - Documentation & Examples (during polish):**
```bash
Task agent T051 "Update CLAUDE.md with new features" &
Task agent T052 "Create dynamic-reload example" &
Task agent T053 "Create health-monitoring example" &
Task agent T054 "Generate updated sample configs" &
wait
```

## Validation Section

✅ **No mis-scoped tasks**: All CORE tasks modify framework only, MODULE tasks stay within module boundaries
✅ **All functionality classified**: Every item from spec has CORE or MODULE designation
✅ **Pattern-first evaluation applied**: Builder options and Observer events used instead of interface changes
✅ **TDD enforced**: All implementation tasks (T023-T046) have prerequisite test tasks (T004-T022)
✅ **Parallel independence verified**: Tasks marked [P] work on different files
✅ **No interface widening**: All enhancements use additive patterns (options, events, new narrow interfaces)

## Notes

- Use build tags `// +build failing_test` for tests until implementation is ready
- Maintain atomic commits showing test → implementation progression
- Performance targets from spec: bootstrap <150ms P50, service lookup <2µs P50, reload <80ms P50
- All new exported symbols must have GoDoc comments
- Observer events must include structured logging with `module`, `phase`, `event` fields
- Error messages follow format: `area: description` (lowercase, no capitals)