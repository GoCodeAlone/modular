# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Execution Flow (main)
```
1. Load plan.md from feature directory
   → If not found: ERROR "No implementation plan found"
   → Extract: tech stack, libraries, structure
2. Load optional design documents:
   → data-model.md: Extract entities → model tasks
   → contracts/: Each file → contract test task
   → research.md: Extract decisions → setup tasks
3. Generate tasks by category:
   → Setup: project init, dependencies, linting
   → Tests: contract tests, integration tests
   → Core: models, services, CLI commands
   → Integration: DB, middleware, logging
   → Polish: unit tests, performance, docs
   → Pattern: builder option additions, observer event emission tests (ensure failing first)
4. Apply task rules:
   → Different files = mark [P] for parallel
   → Same file = sequential (no [P])
   → Tests before implementation (TDD)
5. Number tasks sequentially (T001, T002...)
6. Generate dependency graph
7. Create parallel execution examples
8. Validate task completeness:
   → All contracts have tests?
   → All entities have models?
   → All endpoints implemented?
9. Return: SUCCESS (tasks ready for execution)
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions
- **Go Project (default)**: DDD layout (see plan) with `cmd/`, `internal/{domain,application,interfaces,infrastructure,platform}`, optional `pkg/`, optional `test/` for cross-package integration/e2e; ordinary tests co-located as `*_test.go`.
- **Web app**: `backend/` (embedded Go Project structure) + `frontend/` (`src/`, `public/`, `tests/`).
- **Mobile**: `api/` (Go Project) + `ios/` or `android/` client.
- Adjust all generated paths based on actual structure decision recorded in `plan.md`.

## Phase 3.1: Setup (Template)
Examples (replace with concrete tasks):
- T001 Create/verify Go module and dependency boundaries.
- T002 [P] Add lint/vet/format targets (Makefile, CI) and minimal README.
- T003 [P] Generate baseline config samples under `configs/`.

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE Core
Guidelines:
- All domain, contract (API), and primary use case tests must exist & intentionally FAIL (RED) prior to writing production logic.
- Tests MUST compile and run; failure must come from assertions (not panics unrelated to target behavior).
- PROHIBITED: `t.Skip`, commented-out assertions, placeholder bodies, "TODO/FIXME/placeholder/future implementation" markers in assertion sections, empty test functions.
- Create the smallest meaningful failing assertion that expresses the desired behavior (e.g., expected value vs zero value, expected error vs nil).
- Use table-driven style for enumerated scenarios; each row should have at least one concrete assertion.
Example placeholders to replace with concrete names when generating tasks:
- T010 [P] Contract test for <endpoint/action> in `internal/interfaces/http/<feature>_test.go`.
- T011 [P] Domain aggregate invariant tests in `internal/domain/<context>/aggregate_test.go`.
- T012 [P] Application use case test in `internal/application/<feature>/usecase_test.go`.
- T013 [P] Repository port behavior test (interface expectations) in `internal/domain/<context>/repository_test.go`.
- T014 [P] Observer event emission test `<event_name>` in `internal/platform/observer/<event>_test.go`.
- T015 [P] Builder option behavior test `<OptionName>` in `internal/<module>/builder_options_test.go`.

## Phase 3.3: Core Implementation (Only after failing tests present)
Implement minimal code to satisfy tests in 3.2. Typical buckets:
- Domain entities/value objects & invariants.
- Application use cases (orchestrating domain + ports).
- Interface adapters (HTTP handlers, CLI commands) – thin.
- Repository interfaces already defined; implementations deferred to Integration.
- Builder options implemented minimally (no side effects until final Build/Start).
- Observer event publishing code added only after emission tests exist.

## Phase 3.4: Integration / Adapters
Add concrete infrastructure & cross-cutting concerns:
- Persistence adapters (DB, migrations) in `internal/infrastructure/persistence/`.
- External service clients, cache, messaging.
- Observability wiring (logging, tracing, metrics) in `internal/platform/`.
- Observer registration & lifecycle integration.
- Config loading & validation.

## Phase 3.5: Hardening & Polish
- Additional edge-case & property tests.
- Performance / load validation (thresholds from plan).
- Security review (timeouts, input validation, error wrapping).
- Documentation updates & sample configs regeneration.
- Refactor duplication (rule of three) & finalize public API surface.
- Confirm no interface widening slipped in without adapter + deprecation.
- Validate event schema stability (no late changes without test updates).

## Phase 3.6: Test Finalization (Placeholder / Skip Elimination)
Purpose: Ensure no latent placeholders remain and all originally deferred scenarios now assert real behavior.
- Scan all test files for markers: `TODO`, `FIXME`, `SKIP`, `t.Skip`, `t.Skipf`, `placeholder`, `future implementation`.
- Replace each with concrete test logic or remove if obsolete (document rationale in commit message if removed).
- Ensure every scenario previously outlined in spec/plan has an asserting test (no silent omission).
- Verify no test relies on sleep-based timing without justification (use deterministic synchronization where possible).
- Confirm code coverage for critical paths (domain invariants, error branches, boundary conditions) — add tests where gaps exist.

## Dependencies (Template Rules)
- All contract & domain tests precede related implementation tasks.
- Domain layer precedes application (use case) layer; application precedes interface/delivery.
- Infrastructure adapters depend on repository interfaces & domain types.
- Cross-cutting (observability, config) after first vertical slice is green.
- Performance & polish after functional correctness.
- Test Finalization (Phase 3.6) after Hardening & Polish tasks that introduce new functionality, but before release tagging / final docs.

## Parallel Example (Illustrative)
```
T010 Contract test <endpoint A> (internal/interfaces/http/a_test.go)
T011 Contract test <endpoint B> (internal/interfaces/http/b_test.go)
T012 Domain invariants (internal/domain/bc/entity_test.go)
T013 Use case test (internal/application/feature/usecase_test.go)
```

## Notes
- Mark [P] only when file paths & data dependencies are isolated.
- Ensure commit history shows RED → GREEN → REFACTOR pattern.
- Prefer early vertical slice to reduce integration risk.
- Avoid speculative abstractions; wait for repetition (≥3 occurrences).

## Task Generation Rules
*Applied during main() execution*

1. **From Contracts**:
   - Each contract file → contract test task [P].
   - Each specified interaction → implementation task (post-test).

2. **From Domain Model**:
   - Each aggregate/value object → domain implementation task (after test).
   - Repository ports defined before infrastructure adapters.

3. **From User Stories / Use Cases**:
   - Each story → use case test + implementation pair.
   - Edge/error scenarios → separate tests.

4. **Ordering (Template)**:
   - Setup → Contract & Domain Tests → Domain Impl → Use Case Tests → Use Case Impl → Interface Adapters → Infrastructure Adapters → Cross-Cutting → Hardening.
   - No implementation before failing test exists.
   - Pattern tasks (builder option tests, observer event tests) precede related implementation.
   - Any interface change triggers: deprecation task, adapter task, migration doc task.

## Validation Checklist
*GATE: Checked by main() before returning*

- [ ] All contracts have corresponding tests
- [ ] All entities have model tasks
- [ ] All tests come before implementation
- [ ] Parallel tasks truly independent
- [ ] Each task specifies exact file path
- [ ] No task modifies same file as another [P] task
- [ ] No remaining TODO/FIXME/placeholder/skip markers in tests (unless explicitly justified)
- [ ] All tests fail first then pass after implementation (TDD evidence in VCS history)
- [ ] All interface changes have adapter + deprecation + migration task
- [ ] Builder options introduced via non-breaking additive methods
- [ ] Observer events have emission + test + documentation task
- [ ] No interface widening without recorded justification