# Implementation Plan: Baseline Modular Framework & Modules

**Branch**: `001-baseline-specification-for` | **Date**: 2025-09-06 | **Spec**: `spec.md`
**Input**: Feature specification from `/specs/001-baseline-specification-for/spec.md`

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
Provide a production-ready modular application framework enabling deterministic lifecycle management, multi-source configuration with provenance, multi-tenancy isolation, dynamic (opt-in) configuration reload, structured lifecycle events, health aggregation, and a baseline suite of pluggable modules (auth, cache, DB, HTTP server/client, reverse proxy, scheduler, event bus, JSON schema, ACME). Research confirms feasibility with clarified performance and governance constraints.

## Technical Context
**Language/Version**: Go 1.23+ (toolchain 1.24.2)  
**Primary Dependencies**: Standard library + selective: chi (router), sql drivers (pgx, mysql, sqlite), redis (optional cache), ACME client libs, JWT/OIDC libs.  
**Storage**: PostgreSQL primary; MySQL/MariaDB, SQLite for dev/test.  
**Testing**: `go test` with integration and module-specific suites; contract tests derived from conceptual contracts.  
**Target Platform**: Linux/macOS server environments (container-friendly).  
**Project Type**: Single backend framework (library-first).  
**Performance Goals**: Bootstrap <200ms (10 modules); config load <2s (1000 fields); O(1) service lookup.  
**Constraints**: Deterministic lifecycle; no global mutable state leaking across tenants; dynamic reload only for tagged fields.  
**Scale/Scope**: 100 active tenants baseline (functional up to 500); up to 500 services registered per process.

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (core framework + modules under mono repo) within existing structure.
- Using framework directly: Yes; modules implement interfaces directly.
- Single data model: Conceptual entity set only; no extraneous DTO layer planned.
- Avoiding patterns: No Repository/UoW; direct driver usage acceptable.

**Architecture**:
- Library-first: Framework core + modular packages.
- Libraries (conceptual): core (lifecycle/config), auth, cache, database, httpserver, httpclient, reverseproxy, scheduler, eventbus, jsonschema, letsencrypt.
- CLI: `modcli` supplies generation & scaffolding.
- Docs: Existing README + spec-driven artifacts; LLM context file maintained via update script.

**Testing (NON-NEGOTIABLE)**:
- TDD sequence enforced: Contract (conceptual) → integration → unit.
- Failing tests precede implementation for new behaviors.
- Real dependencies: Use real DB (Postgres) & in-memory alt where needed.
- Integration tests: Required for new module types & registry behaviors.
- No skipping RED phase; enforced via review.

**Observability**:
- Structured logging: Yes (fields for module, phase, correlation).
- Unified stream: Backend only (no frontend scope here).
- Error context: Wrapped with category + cause.

**Versioning**:
- SemVer followed; modules declare minimal core version.
- Breaking changes gated by deprecation notice (≥1 minor release).
- Build metadata handled by release tooling.

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

### Source Code (repository root)
```
# Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure]
```

**Structure Decision**: Option 1 (single project/library-first) retained.

## Phase 0: Outline & Research
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

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

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

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

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
No violations requiring justification; single-project model maintained.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - approach documented)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*