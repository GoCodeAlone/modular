# Break down the plan into executable tasks


Break down the plan into executable tasks.

This is the third step in the Spec-Driven Development lifecycle.

Given the context provided as an argument, do this:

1. Run `scripts/check-task-prerequisites.sh --json` from repo root and parse FEATURE_DIR and AVAILABLE_DOCS list. All paths must be absolute.
2. Load and analyze available design documents:
   - Always read plan.md for tech stack and libraries
   - IF EXISTS: Read data-model.md for entities
   - IF EXISTS: Read contracts/ for API endpoints  
   - IF EXISTS: Read research.md for technical decisions
   - IF EXISTS: Read quickstart.md for test scenarios
    - Identify for every described functionality whether it is classified as CORE (belongs in repository root framework) or MODULE (belongs under `modules/<name>/`).
       * CORE: lifecycle orchestration, configuration system, service registry, tenant/instance context, lifecycle events dispatcher, health aggregation.
       * MODULE: auth, cache, database drivers, http server/client adapters, reverse proxy, scheduler jobs, event bus implementations, certificate/ACME management, JSON schema validation, routing integrations, logging decorators.
       * If any functionality lacks classification → ERROR "Unclassified functionality: <item>".
   
   Note: Not all projects have all documents. For example:
   - CLI tools might not have contracts/
   - Simple libraries might not need data-model.md
   - Generate tasks based on what's available

3. Generate tasks following the template:
   - Use `/templates/tasks-template.md` as the base
   - Replace example tasks with actual tasks based on:
     * **Setup tasks**: Project init, dependencies, linting
     * **Test tasks [P]**: One per contract, one per integration scenario
     * **Core tasks**: One per entity, service, CLI command, endpoint
     * **Integration tasks**: DB connections, middleware, logging
     * **Polish tasks [P]**: Unit tests, performance, docs
       * Each task MUST include a `[CORE]` or `[MODULE:<module-name>]` tag prefix before the description.
          - Example: `T012 [CORE][P] Implement service registry entry struct in service_registry_entry.go`
          - Example: `T039 [MODULE:auth] Implement JWT validator in modules/auth/jwt_validator.go`

4. Task generation rules:
   - Each contract file → contract test task marked [P]
   - Each entity in data-model → model creation task marked [P]
   - Each endpoint → implementation task (not parallel if shared files)
   - Each user story → integration test marked [P]
   - Different files = can be parallel [P]
   - Same file = sequential (no [P])
   - CORE tasks may not introduce or modify files inside `modules/` (enforce separation) → if violation detected: ERROR "Core task mis-scoped: <task id>"
   - MODULE tasks must write only inside `modules/<module>/` (except tests placed in module or shared test helpers) → else ERROR "Module task mis-scoped: <task id>"

5. Order tasks by dependencies:
   - Setup before everything
   - Tests before implementation (TDD)
   - Models before services
   - Services before endpoints
   - Core before integration
   - Everything before polish

6. Include parallel execution examples:
   - Group [P] tasks that can run together
   - Show actual Task agent commands

7. Create FEATURE_DIR/tasks.md with:
   - Correct feature name from implementation plan
   - Numbered tasks (T001, T002, etc.)
   - Clear file paths for each task
   - Dependency notes
   - Parallel execution guidance
   - A classification summary table listing counts of CORE vs MODULE tasks
   - A validation section stating: no mis-scoped tasks, all functionality classified

Context for task generation: $ARGUMENTS

The tasks.md should be immediately executable - each task must be specific enough that an LLM can complete it without additional context.
