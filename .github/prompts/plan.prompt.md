# Plan how to implement the specified feature


Plan how to implement the specified feature.

This is the second step in the Spec-Driven Development lifecycle.

Given the implementation details provided as an argument, do this:

1. Run `scripts/setup-plan.sh --json` from the repo root and parse JSON for FEATURE_SPEC, IMPL_PLAN, SPECS_DIR, BRANCH. All future file paths must be absolute.
2. Read and analyze the feature specification to understand:
   - The feature requirements and user stories
   - Functional and non-functional requirements
   - Success criteria and acceptance criteria
   - Any technical constraints or dependencies mentioned

3. Read the constitution at `/memory/constitution.md` to understand constitutional requirements.
   - Validate the specification includes a Scope Classification section produced by the spec step; ERROR if missing.
   - Parse CORE vs MODULE counts; if any MODULE item overlaps a defined CORE area (lifecycle, registry, configuration, multi-tenancy context, lifecycle events, health, error taxonomy) → ERROR "Module encroaches on core: <item>".
   - Extract proposed public API changes (new exported symbols, interface or constructor mutations). For each, evaluate Builder and Observer alternatives (Articles XII & XVI). If mutation lacks evaluation → ERROR "Missing pattern evaluation for API change: <symbol>".
   - If >2 domain entities and no glossary/ bounded context section is present → WARN "Missing bounded context glossary".

4. Execute the implementation plan template:
   - Load `/templates/plan-template.md` (already copied to IMPL_PLAN path)
   - Set Input path to FEATURE_SPEC
   - Run the Execution Flow (main) function steps 1-10
   - The template is self-contained and executable
   - Follow error handling and gate checks as specified
   - Let the template guide artifact generation in $SPECS_DIR:
     * Phase 0 generates research.md
     * Phase 1 generates data-model.md, contracts/, quickstart.md
     * Phase 2 generates tasks.md
    - Incorporate user-provided details from arguments into Technical Context: $ARGUMENTS
    - In Technical Context add a "Scope Enforcement" subsection summarizing:
       * List of CORE components (from spec) that will remain in root
       * List of MODULE components with their module directories
       * Any contested items resolved with rationale
    - During Phase 1 generation ensure contracts/data-model segregate CORE vs MODULE types (e.g., do not add auth-specific entities to core data-model). If violation detected during extraction → ERROR "Scope violation in design artifact: <file> <description>".
   - During Phase 1 capture API evolution decisions (Builder option list, Observer events list, adapters needed). Persist in plan.
   - Update Progress Tracking as you complete each phase

5. Verify execution completed:
   - Check Progress Tracking shows all phases complete
   - Ensure all required artifacts were generated
   - Confirm no ERROR states in execution

 6. Report results with branch name, file paths, generated artifacts, and CORE vs MODULE counts reaffirmed.
  
 7. Validate consistency across prompts:
      - Ensure same error phrase prefixes used: "ERROR" (all caps) followed by colon.
      - Ensure scope related errors use one of:
         * "Module encroaches on core: <item>"
         * "Scope violation in design artifact: <file> <description>"
         * "Missing Scope Classification section in spec"
      - If inconsistency found → emit ERROR summary and abort.

Use absolute paths with the repository root for all file operations to avoid path issues.
