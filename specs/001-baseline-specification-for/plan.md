# Implementation Plan: Baseline Specification Enablement (Dynamic Reload, Health Aggregation & Supporting Enhancements)

**Branch**: `001-baseline-specification-for` | **Date**: 2025-09-07 | **Spec**: `specs/001-baseline-specification-for/spec.md`
**Input**: Baseline framework capability specification (FR-001..FR-050) focusing on remaining Planned items.

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
4. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
5. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, or `GEMINI.md` for Gemini CLI).
6. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
7. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
8. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Establish an implementation pathway to close all Planned gaps in the baseline spec with minimal disruption to existing public APIs. Two flagship capabilities drive most structural work:
1. Dynamic Configuration Reload (FR-045) – selective hot reload for fields tagged dynamic, with a `Reloadable` interface and observer events.
2. Aggregate Health & Readiness (FR-048) – uniform tri-state health collection (healthy|degraded|unhealthy) plus readiness rules excluding optional module failures.

Supporting enhancements (FR-005, FR-014, FR-019, FR-023, FR-032, FR-038, FR-039, FR-044, FR-046, FR-049) will be addressed via additive builder options, narrow new interfaces, and observer events, preserving Article XII contract stability.

Key architectural principle: prefer Builder / Observer evolution; avoid interface widening of core `Module` or central application types. All new functionality introduced as opt-in, defaulting to current behavior.

Scope Classification:
- Core Framework Changes: dynamic reload pipeline, health aggregator service, service scope enum, tie-break priority metadata, error taxonomy unification, secret classification model, optional scheduler catch-up policy integration.
- Module Enhancements: auth OIDC provider SPI, ACME escalation telemetry, scheduler policy config exposure, decorator ordering docs/tests.
- Cross-Cutting Observability: new lifecycle/health/reload observer events and metrics.

Out-of-Scope (explicit): Introducing new transport protocols, persistence engines beyond current list, multi-process orchestration, UI tooling.

## Technical Context
**Language/Version**: Go (toolchain 1.24.x, module go 1.23 listed)  
**Primary Dependencies**: `chi` (router via chimux module), standard library crypto/net/http/time, ACME/Let's Encrypt client, SQL drivers (pgx / mysql / sqlite), Redis client (cache), cron scheduler library (internal or third-party)  
**Storage**: External DBs (PostgreSQL primary), Redis (cache), TLS cert storage (filesystem)  
**Testing**: `go test` with BDD/integration suites already present; new enhancements will add focused unit + integration tests; optional benchmarks for bootstrap & lookup  
**Target Platform**: Linux/Windows server processes (no frontend/mobile split)  
**Project Type**: Single Go backend (Option 1 structure retained)  
**Performance Goals**: From spec success criteria (bootstrap <150ms P50, service lookup <2µs P50, reload <80ms P50)  
**Constraints**: Maintain O(1) service registry lookups; avoid global locks on hot paths; no additional mandatory external dependencies  
**Scale/Scope**: 100 active tenants baseline (functional to 500), up to 500 services; scheduler backlog policies bounded (time or count)

Unresolved Unknowns: None (all clarifications incorporated). Research phase documents decisions & alternatives.

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: [#] (max 3 - e.g., api, cli, tests)
- Using framework directly? (no wrapper classes)
- Single data model? (no DTOs unless serialization differs)
- Avoiding patterns? (no Repository/UoW without proven need)

**Architecture**:
- EVERY feature as library? (no direct app code)
- Libraries listed: [name + purpose for each]
- CLI per library: [commands with --help/--version/--format]
- Library docs: llms.txt format planned?

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? (test MUST fail first)
- Git commits show tests before implementation?
- Order: Contract→Integration→E2E→Unit strictly followed?
- Real dependencies used? (actual DBs, not mocks)
- Integration tests for: new libraries, contract changes, shared schemas?
- FORBIDDEN: Implementation before test, skipping RED phase

**Observability**:
- Structured logging included?
- Frontend logs → backend? (unified stream)
- Error context sufficient?

**Versioning**:
- Version number assigned? (MAJOR.MINOR.BUILD)
- BUILD increments on every change?
- Breaking changes handled? (parallel tests, migration plan)

**Public API Stability & Review (Article XII)**:
- Any new exported symbols? (list & rationale)
- Added methods to existing interfaces? (FORBIDDEN unless deprecation + adapter path defined)
- Constructor / interface change proposed? (justify why NOT solved via Builder option or Observer event)
- Deprecations annotated with proper comment form?
- Migration notes required? (link or state N/A)

**Strategic Patterns & DDD (Article XVI)**:
- Bounded contexts identified? (name each)
- Domain glossary established? (central term list planned)
- Builder options to be added (list names + defaults + backward compat note)
- Observer events to add (name, payload schema, emission timing) & tests planned?
- Interface widening avoided? (if not, justification & adapter strategy)
- Anti-corruption layers required? (list external systems or N/A)
- Ubiquitous language applied across config/logging/service names?

**Performance & Operational Baselines** (cross-check with Constitution Articles X & XVI linkage):
- Startup impact estimated? (<200ms target unaffected or measurement plan)
- Service lookup complexity unchanged (O(1))?
- Config field count increase risk assessed (provenance & validation impact)?

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

## Constitution Check
*Initial Assessment (Pre-Design Implementation)*

**Simplicity**:
- Projects: 1 core + modules + CLI (within allowed maximum). PASS
- Direct framework usage preserved (no wrapper layer). PASS
- Data model additive only (enums/interfaces). PASS
- Avoided heavy patterns (Repository already limited to DB module; not expanding). PASS

**Architecture**:
- Features delivered as internal packages / additive module options. PASS
- Libraries (conceptual): core (lifecycle/config), dynamicreload (new internal pkg), health (new internal pkg), auth (existing extended), scheduler (extended). PASS
- CLI changes limited to generating updated config samples (no new subcommands yet). PASS
- AI/LLM assistant context file update planned after Phase 1 (keep <150 lines). PASS

**Testing**:
- Commit discipline: Plan mandates failing tests first; we will stage failing tests under build tag until ready (avoid breaking main). CONDITIONAL PASS
- Order: Contract (interfaces & events) → integration tests → benchmarks. PASS
- Real dependencies: use in-memory + real DB/Redis already done; new tests reuse existing harness. PASS
- No implementation before tests (enforced at per-feature PR). PASS

**Observability**:
- Structured logging continues; new events (ConfigReload*, HealthSnapshot*). PASS
- Error context unaffected. PASS

**Versioning**:
- Additions only; no breaking removal. PASS
- Migration notes: none required (no deprecated symbols yet). PASS

**Public API Stability (Article XII)**:
- New exported symbols (planned): `ServiceScope` enum, `Reloadable` interface, `HealthReporter` interface, `AggregateHealthService` accessor func, error category constants, secret classification constants.
- No existing interface widened; all additive. PASS
- Constructor changes avoided; builder/options pattern for enabling reload & health aggregator. PASS

**Strategic Patterns (Article XVI)**:
- Builder options (planned): `WithDynamicReload()`, `WithHealthAggregator()`, `WithSchedulerCatchUp(policy)`, `WithServiceScope(scope)`, `WithTenantGuardMode(mode)`.
- Observer events (planned): `ConfigReloadStarted`, `ConfigReloadCompleted`, `HealthEvaluated`, `CertificateRenewalEscalated`.
- Interface widening avoided. PASS
- Bounded contexts: lifecycle, configuration, reload, health, auth, scheduler, secrets.

**Performance & Operational Baselines**:
- Startup impact: reload & health components lazy-init; negligible (<5ms target). PASS
- Service lookup remains O(1) (no change to map structure). PASS
- Config validation overhead: dynamic tagging parse one-time; diff cost proportional to changed fields only. PASS

Initial Constitution Check: PASS (no violations requiring Complexity Tracking)
# Option 2: Web application (frontend + backend)
# Embed the Go Project structure inside backend/; frontend follows its ecosystem conventions.
backend/
   cmd/
      <app-name>/main.go
   internal/
      domain/
      application/
      interfaces/
         http/
         cli/
      infrastructure/
      platform/
   pkg/
   configs/
   docs/
   tools/
   test/                   # Optional integration/e2e for backend

frontend/
   src/
      components/
      pages/ (or routes/ per framework)
      services/
      lib/
   public/
   tests/
   package.json (or equivalent)

# Option 3: Mobile + API (when "iOS/Android" detected)
api/                      # Same structure as Option 1 (Go Project)
   cmd/
   internal/
   pkg/
   test/

ios/ or android/          # Platform-specific client implementation
   <standard platform layout>
```

**Structure Decision**: Option 1 (Go Project) retained; no frontend/mobile split introduced.

## Phase 0: Outline & Research (Completed)
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md (created) – all clarifications resolved; decisions & alternatives recorded.

## Phase 1: Design & Contracts (Completed)
*Prerequisites: research.md complete (met)*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `/scripts/update-agent-context.sh [claude|gemini|copilot]` for your AI assistant
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*.md, quickstart.md. Failing tests deferred until /tasks phase (will add with build tag to avoid destabilizing main). Agent context update deferred to tasks execution.

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
- Load `/templates/tasks-template.md` as base
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- Each contract → contract test task [P]
- Each entity → model creation task [P] 
- Each user story → integration test task
- Implementation tasks to make tests pass

**Ordering Strategy**:
- TDD order: Tests before implementation 
- Dependency order: Models before services before UI
- Mark [P] for parallel execution (independent files)

**Estimated Output**: 25-30 numbered, ordered tasks in tasks.md

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [ ] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (N/A)

---
*Based on Constitution v1.2.0 - See `/memory/constitution.md`*