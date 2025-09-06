# Tasks: Baseline Modular Framework

**Feature Directory**: `/Users/jlangevin/Projects/modular/specs/001-baseline-specification-for`
**Input Docs**: plan.md, research.md, data-model.md, quickstart.md, contracts/*.md
**Project Structure Mode**: Single project (library-first) per plan.md

## Legend
- Format: `T### [P?] Description`
- [P] = May run in parallel (different files, no dependency ordering)
- Omit [P] when sequential ordering or same file/structural dependency exists
- All test tasks precede implementation tasks (TDD mandate)

## Phase 3.1: Setup
1. T001 Initialize task scaffolding context file `internal/dev/tasks_context.go` (records feature id & version for tooling) 
2. T002 Create placeholder test directory structure: `tests/contract/`, `tests/integration/`, ensure `go.mod` untouched
3. T003 [P] Add make target `tasks-check` in `Makefile` to run lint + `go test ./...` (idempotent)
4. T004 [P] Add README section "Baseline Framework Tasks" referencing this tasks.md (edit `DOCUMENTATION.md`)

## Phase 3.2: Contract & Integration Tests (Write failing tests first)
5. T005 [P] Auth contract test skeleton in `tests/contract/auth_contract_test.go` validating operations Authenticate/ValidateToken/RefreshMetadata (currently unimplemented -> expected failures)
6. T006 [P] Configuration contract test skeleton in `tests/contract/config_contract_test.go` covering Load/Validate/GetProvenance/Reload error paths
7. T007 [P] Service registry contract test skeleton in `tests/contract/registry_contract_test.go` covering Register/ResolveByName/ResolveByInterface ambiguity + duplicate cases
8. T008 [P] Scheduler contract test skeleton in `tests/contract/scheduler_contract_test.go` covering Register duplicate + invalid cron, Start/Stop sequencing
9. T009 [P] Lifecycle events contract test skeleton in `tests/contract/lifecycle_events_contract_test.go` ensuring all phases emit events (observer pending)
10. T010 [P] Health aggregation contract test skeleton in `tests/contract/health_contract_test.go` verifying worst-state and readiness exclusion logic
11. T011 Integration quickstart test in `tests/integration/quickstart_flow_test.go` simulating quickstart.md steps (will fail until implementations exist)

## Phase 3.3: Core Models (Entities from data-model.md)
12. T012 [P] Implement `Application` core struct skeleton in `application_core.go` (fields only, no methods)
13. T013 [P] Implement `Module` struct skeleton in `module_core.go` (fields: Name, Version, DeclaredDependencies, ProvidesServices, ConfigSpec, DynamicFields)
14. T014 [P] Implement `ConfigurationField` + provenance structs in `config_types.go`
15. T015 [P] Implement `TenantContext` and `InstanceContext` in `context_scopes.go`
16. T016 [P] Implement `ServiceRegistryEntry` struct in `service_registry_entry.go`
17. T017 [P] Implement `LifecycleEvent` struct in `lifecycle_event_types.go`
18. T018 [P] Implement `HealthStatus` struct in `health_types.go`
19. T019 [P] Implement `ScheduledJobDefinition` struct in `scheduler_types.go`
20. T020 [P] Implement `EventMessage` struct in `event_message.go`
21. T021 [P] Implement `CertificateAsset` struct in `certificate_asset.go`

## Phase 3.4: Core Services & Interfaces
22. T022 Define (or confirm existing) auth interfaces in `modules/auth/interfaces.go` (Authenticate, ValidateToken, RefreshMetadata) without implementation (module-scoped)
23. T023 Define configuration service interfaces in `config/interfaces.go`
24. T024 Define health service interfaces in `health/interfaces.go`
25. T025 Define lifecycle event dispatcher interface in `lifecycle/interfaces.go`
26. T026 Define scheduler interfaces in `scheduler/interfaces.go`
27. T027 Define service registry interface in `registry/interfaces.go`

## Phase 3.5: Service Implementations (Make tests pass gradually)
28. T028 Implement minimal failing auth service stub in `modules/auth/service.go` returning explicit TODO errors (replace progressively)
29. T029 Implement configuration loader skeleton in `config/loader.go` with stubbed methods
30. T030 Implement service registry core map-based structure in `registry/registry.go` (Register/Resolve methods returning not implemented errors initially)
31. T031 Implement lifecycle event dispatcher stub in `lifecycle/dispatcher.go`
32. T032 Implement health aggregator stub in `health/aggregator.go`
33. T033 Implement scheduler stub in `scheduler/scheduler.go`

## Phase 3.6: Incremental Feature Completion (Turn stubs into logic)
34. T034 Service registry: support registration, duplicate detection, O(1) lookup by name/interface in `registry/registry.go`
35. T035 Service registry: implement tie-break (explicit name > priority > registration time) + ambiguity error formatting
36. T036 Configuration: implement defaults application + required field validation in `config/loader.go`
37. T037 Configuration: implement provenance tracking & secret redaction utility in `config/provenance.go`
38. T038 Configuration: implement dynamic reload path & validation re-run
39. T039 Auth: implement JWT validation (HS256/RS256) in `modules/auth/jwt_validator.go`
40. T040 Auth: implement OIDC metadata fetch + JWKS refresh in `modules/auth/oidc.go`
41. T041 Auth: implement API Key header authenticator in `modules/auth/apikey.go`
42. T042 Auth: principal model & claims mapping in `modules/auth/principal.go`
43. T043 Lifecycle dispatcher: emit events & buffering/backpressure warning in `lifecycle/dispatcher.go`
44. T044 Health: implement aggregation worst-case logic & readiness exclusion in `health/aggregator.go`
45. T045 Scheduler: parse cron (use robfig/cron v3), enforce maxConcurrency + bounded backfill in `modules/scheduler/scheduler.go`
46. T046 Scheduler: backfill policy enforcement logic & tests update in `modules/scheduler/scheduler.go`
47. T047 Certificate renewal logic skeleton in `modules/letsencrypt/manager.go`
48. T048 Certificate renewal: implement 30-day pre-renew & 7-day escalation in `modules/letsencrypt/manager.go`
49. T049 Event bus minimal dispatch interface & in-memory implementation in `modules/eventbus/eventbus.go`

## Phase 3.7: Integration Wiring
50. T050 Application: implement deterministic start order and reverse stop in `application_lifecycle.go`
51. T051 Application: integrate configuration load + validation gate before module start
52. T052 Application: integrate service registry population from modules
53. T053 Application: integrate lifecycle dispatcher & health aggregation hooks
54. T054 Application: integrate scheduler start/stop and graceful shutdown
55. T055 Application: integrate auth & event bus optional module registration patterns

## Phase 3.8: Quickstart Pass & End-to-End
56. T056 Implement quickstart scenario harness in `tests/integration/quickstart_flow_test.go` to pass with real stubs replaced
57. T057 Add integration test for dynamic config reload in `tests/integration/config_reload_test.go`
58. T058 Add integration test for tenant isolation in `tests/integration/tenant_isolation_test.go`
59. T059 Add integration test for scheduler bounded backfill `tests/integration/scheduler_backfill_test.go`
60. T060 Add integration test for certificate renewal escalation `tests/integration/cert_renewal_test.go`

## Phase 3.9: Polish & Performance
61. T061 [P] Add unit tests for service registry edge cases `tests/unit/registry_edge_test.go`
62. T062 [P] Add performance benchmarks for service registry lookups `service_registry_benchmark_test.go` (core registry benchmark lives at root)
63. T063 [P] Add configuration provenance unit tests `tests/unit/config_provenance_test.go`
64. T064 [P] Add auth mechanism unit tests (JWT, OIDC, API key) in `modules/auth/auth_mechanisms_test.go`
65. T065 [P] Add health aggregation unit tests `tests/unit/health_aggregation_test.go`
66. T066 [P] Optimize registry hot path (pre-sized maps) & document results in `DOCUMENTATION.md`
67. T067 [P] Update `GO_BEST_PRACTICES.md` with performance guardrail validation steps
68. T068 Run full lint + tests + benchmarks; capture baseline numbers in `performance/baseline.md`
69. T069 Final documentation pass: update `DOCUMENTATION.md` Quickstart verification section
70. T070 Cleanup: remove TODO comments from stubs and ensure exported API docs present

## Dependencies & Ordering Notes
- T005-T011 must be created before any implementation (T012+)
- Model structs (T012-T021) must precede interface definitions (T022-T027) only for referencing types
- Interfaces precede service stubs (T028-T033)
- Stubs (T028-T033) precede logic completion tasks (T034-T049)
- Application wiring (T050-T055) depends on prior implementations
- Quickstart & integration tests (T056-T060) depend on wiring
- Polish tasks (T061-T070) depend on all core + integration functionality

## Parallel Execution Guidance
- Safe initial parallel batch after tests written: T012-T021 (distinct files)
- Logic improvement parallel sets (ensure different files):
  * Batch A: T034, T036, T039, T044, T045
  * Batch B: T035, T037, T041, T047, T049
- Polish parallel batch: T061-T067 (distinct test files + doc edits)

## Validation Checklist
- [ ] All 6 contract files have matching test tasks (T005-T010) ✔
- [ ] Quickstart integration test task present (T011) ✔
- [ ] All 11 entities mapped to model struct tasks (T012-T021) ✔
- [ ] Tests precede implementation ✔
- [ ] Parallel tasks only touch distinct files ✔
- [ ] Performance benchmark task present (T062) ✔
- [ ] Provenance & reload tasks present (T037, T038) ✔
- [ ] Scheduler backfill tasks present (T045, T046) ✔
- [ ] Certificate renewal tasks present (T047, T048) ✔

## Parallel Examples
```
# Example: Run all contract tests creation in parallel
Tasks: T005 T006 T007 T008 T009 T010

# Example: Parallel model struct creation
Tasks: T012 T013 T014 T015 T016 T017 T018 T019 T020 T021

# Example: Performance & polish batch
Tasks: T061 T062 T063 T064 T065 T066 T067
```

---
Generated per tasks.prompt.md Phase 2 rules.

### Scoping Note
Auth, scheduler, event bus, and certificate renewal concerns remain inside their respective existing module directories under `modules/`. Core keeps only generic lifecycle, configuration, health, and registry responsibilities. Paths updated to prevent accidental duplication of module-level functionality in the framework root.
