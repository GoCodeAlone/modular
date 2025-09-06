# Constitution Update Checklist

When amending the constitution (`/memory/constitution.md`), ensure all dependent documents are updated to maintain consistency.

## Templates to Update

### When adding/modifying ANY article:
- [ ] `/templates/plan-template.md` - Update Constitution Check section
- [ ] `/templates/spec-template.md` - Update if requirements/scope affected
- [ ] `/templates/tasks-template.md` - Update if new task types needed
- [ ] `/.github/prompts/plan.prompt.md` - Update if planning process changes
- [ ] `/.github/prompts/specify.prompt.md` - Update if specification generation affected
- [ ] `/.github/prompts/tasks.prompt.md` - Update if task generation affected
- [ ] `/.github/copilot-instructions.md` - Update runtime development guidelines

### Article-specific updates:

#### Article I (Library-First):
- [ ] Ensure templates emphasize library creation
- [ ] Update CLI command examples
- [ ] Add llms.txt documentation requirements

#### Article II (CLI Interface):
- [ ] Update CLI flag requirements in templates
- [ ] Add text I/O protocol reminders

#### Article III (Test-First):
- [ ] Update test order in all templates
- [ ] Emphasize TDD requirements
- [ ] Add test approval gates

#### Article IV (Integration Testing):
- [ ] List integration test triggers
- [ ] Update test type priorities
- [ ] Add real dependency requirements

#### Article V (Observability):
- [ ] Add logging requirements to templates
- [ ] Include multi-tier log streaming
- [ ] Update performance monitoring sections

#### Article VI (Versioning):
- [ ] Add version increment reminders
- [ ] Include breaking change procedures
- [ ] Update migration requirements

#### Article VII (Simplicity):
- [ ] Update project count limits
- [ ] Add pattern prohibition examples
- [ ] Include YAGNI reminders

#### Article XI (Idiomatic Go & Boilerplate Minimization):
- [ ] Add reference in README and DOCUMENTATION governance sections
- [ ] Ensure GO_BEST_PRACTICES.md reflects constructor/interface guidance
- [ ] Add boilerplate LOC target note to module template (if exists)

#### Article XII (Public API Stability & Review):
- [ ] Confirm API diff tooling docs up to date (API_CONTRACT_MANAGEMENT.md)
- [ ] Add deprecation comment pattern to templates
- [ ] Update PR checklist to require rationale for each new exported symbol

#### Article XIII (Documentation & Example Freshness):
- [ ] Verify examples compile after changes
- [ ] Add doc-update requirement to contribution docs
- [ ] Add future automation placeholder issue (doc drift)

#### Article XIV (Boilerplate Reduction Targets):
- [ ] Track minimal module LOC in module generation template / CLI
- [ ] Add justification comment pattern to templates

#### Article XV (Consistency & Style Enforcement):
- [ ] Ensure golangci-lint config enforces logging & error message style (add custom linters if needed)
- [ ] Add structured logging key guidelines to GO_BEST_PRACTICES.md (already present?)
- [ ] Document panic usage policy in templates

## Validation Steps

1. **Before committing constitution changes:**
   - [ ] All templates reference new requirements
   - [ ] Examples updated to match new rules
   - [ ] No contradictions between documents

2. **After updating templates:**
   - [ ] Run through a sample implementation plan
   - [ ] Verify all constitution requirements addressed
   - [ ] Check that templates are self-contained (readable without constitution)

3. **Version tracking:**
   - [ ] Update constitution version number
   - [ ] Note version in template footers
   - [ ] Add amendment to constitution history

## Common Misses

Watch for these often-forgotten updates:
- Command documentation (`/commands/*.md`)
- Checklist items in templates
- Example code/commands
- Domain-specific variations (web vs mobile vs CLI)
- Cross-references between documents

## Template Sync Status

Last sync check: 2025-07-16
- Constitution version: 2.1.1
- Templates aligned: ❌ (missing versioning, observability details)

---

*This checklist ensures the constitution's principles are consistently applied across all project documentation.*