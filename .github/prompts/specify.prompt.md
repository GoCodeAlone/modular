# Start a new feature by creating a specification and feature branch


Start a new feature by creating a specification and feature branch.

This is the first step in the Spec-Driven Development lifecycle.

Given the feature description provided as an argument, do this:

1. Run the script `scripts/create-new-feature.sh --json "$ARGUMENTS"` from repo root and parse its JSON output for BRANCH_NAME and SPEC_FILE. All file paths must be absolute.
2. Load `templates/spec-template.md` to understand required sections.
3. Write the specification to SPEC_FILE using the template structure, replacing placeholders with concrete details derived from the feature description (arguments) while preserving section order and headings.
 4. In the specification, include a "Scope Classification" subsection under Technical Context (or similar) that enumerates every planned functionality item and labels it explicitly as:
	 - CORE: belongs to root framework (lifecycle, service registry, configuration, multi-tenancy context, lifecycle events, health aggregation, shared error taxonomy)
	 - MODULE: belongs to a specific module directory (auth, cache, database, httpserver, httpclient, scheduler, eventbus, reverseproxy, letsencrypt, jsonschema, chimux, logging decorators, etc.)
	 - For each MODULE item, include target module name.
	 - If any functionality cannot be clearly classified, abort with ERROR "Unclassified functionality discovered".
 5. Add an "API Evolution & Patterns" subsection listing anticipated public API changes and, for each: Builder option feasibility, Observer event feasibility, justification if interface change; list new Builder options (name, default), Observer events (name, payload, timing), adapters & deprecations.
 6. Add a "Bounded Context Glossary" subsection if >2 domain entities; if omitted in that case → ERROR "Missing glossary".
 7. Add a "Mis-Scope Guardrails" note listing at least three examples of incorrect placements (e.g., putting JWT parsing in core) and their corrections.
 8. Report completion with branch name, spec file path, summary counts (#CORE, #MODULE), glossary present (Yes/No), API evolution candidates count, and readiness for the next phase.

Note: The script creates and checks out the new branch and initializes the spec file before writing.
