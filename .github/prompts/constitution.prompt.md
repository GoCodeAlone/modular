# Update (Amend) the Project Constitution and Propagate Required Documentation Changes

Update (amend) the living constitution stored at `/memory/constitution.md`, then drive all required follow‑through updates using `/memory/constitution_update_checklist.md`.

This prompt automates the Constitution Evolution workflow.

Given enhancement arguments (the proposed constitutional changes) provided to the command as a single string (`$ARGUMENTS`), perform ALL of the following steps strictly in order. All repository file paths MUST be absolute from repo root. Use the exact error prefixes defined below.

---
## 1. Parse Inputs
1. Treat `$ARGUMENTS` as a YAML or Markdown fragment containing one or more proposed changes. Accept either structured keys or freeform text. Attempt to extract for each proposed change:
   - `title` (short descriptive phrase)
   - `type`: `NEW_ARTICLE` | `AMEND_ARTICLE` | `DEPRECATE_ARTICLE` | `CLARIFICATION` | `GLOSSARY_UPDATE`
   - `target` (Article number / subsection reference if not NEW)
   - `rationale` (problem / motivation)
   - `pattern_impact` (Builder / Observer / Interface / Config / Multi-tenancy / Error Taxonomy)
   - `backwards_compat`: YES/NO + migration window if NO
   - `version_effective` (semantic version or `NEXT.MINOR`, `NEXT.PATCH`)
   - `dependencies` (list of docs or modules impacted)
2. If any change cannot be parsed into at least `type` + (`title` or `target`): `ERROR: Unparseable proposed change` (list offending snippet).

## 2. Load Current Constitution
Read `/memory/constitution.md` fully.
Validate it contains numbered Articles (regex `^## +Article +[IVXLC]+` or `^### +Article`). If missing: `ERROR: Constitution format invalid`.

## 3. Pre‑Validation of Proposed Changes
For each change:
1. If `NEW_ARTICLE` → ensure no existing article with same `title` (case-insensitive). If duplicate: `ERROR: Duplicate article title <title>`.
2. If `AMEND_ARTICLE` or `DEPRECATE_ARTICLE` → ensure target exists. If not: `ERROR: Target article not found <target>`.
3. If `DEPRECATE_ARTICLE` → must include `version_effective` AND `backwards_compat=NO`. If not: `ERROR: Invalid deprecation metadata <target>`.
4. If `type` affects interfaces (pattern_impact includes `Interface`) and rationale does not explicitly state why Builder or Observer patterns (Constitution Articles XII & XVI) are insufficient: `ERROR: Missing pattern evaluation for interface change <target or title>`.
5. If a change attempts to move MODULE concerns (auth, cache, database, httpserver, etc.) into CORE scope (lifecycle, registry, configuration, multi-tenancy, lifecycle events, health, error taxonomy) without rationale showing boundary necessity: `ERROR: Module encroaches on core: <description>`.

## 4. Apply Amendments In-Memory
Perform modifications in memory before writing:
1. NEW_ARTICLE: Append a new Article section at the end. Use canonical heading: `## Article <RomanNumeral>: <Title>` and below it a `Status:` line with one of `Active`, `Deprecated (Effective vX.Y.Z)`, `Pending (Effective next release)`. Choose next Roman numeral sequentially.
2. AMEND_ARTICLE: Insert a `Revision YYYY-MM-DD:` subsection summarizing change, leaving original text intact above unless explicitly replacing a paragraph (then keep original inside a collapsible or quoted block preceded by `Legacy:`). Preserve prior numbering.
3. DEPRECATE_ARTICLE: Add top line `Status: Deprecated (Removal effective <version_effective>)` and append a Migration subsection with required adapters/Builder options/Observer events.
4. CLARIFICATION: Append a `Clarification YYYY-MM-DD:` bullet list at end of the target Article.
5. GLOSSARY_UPDATE: Update (or create if absent) a `### Glossary` section at bottom; merge term definitions alphabetically. If >2 new domain terms introduced across changes and no glossary previously existed → WARN (not ERROR) but still create it.

## 5. Cross-Reference & Pattern Enforcement
1. For each interface-impacting change ensure one of:
   - A new Builder option (record name & default)
   - A new Observer event (name, payload, emission timing)
   - OR explicit justification line: `Justification: Builder/Observer insufficient because ...`
   Otherwise: `ERROR: Missing pattern evaluation for API change: <symbol or article>`.
2. Collect all new Builder options & Observer events into a consolidated `### Pattern Evolution Ledger` section (create if missing) with dated entries.

## 6. Write Updated Constitution
After successful validation write back to `/memory/constitution.md` with amendments applied. Ensure file ends with newline and no trailing whitespace on lines.

## 7. Update the Constitution Update Checklist
1. Load `/memory/constitution_update_checklist.md`.
2. Append a new dated block:
   - `Date:` current date (UTC)
   - `Changes:` bullet list referencing Article numbers / titles and change types
   - `Required Docs:` derived from aggregated `dependencies` plus automatically inferred: if pattern impacts include `Observer` add `OBSERVER_PATTERN.md`; if `Builder` add `API_CONTRACT_MANAGEMENT.md`; if `Error Taxonomy` add `errors.go` and any error docs; if `Multi-tenancy` add `CONCURRENCY_GUIDELINES.md` or multi-tenant docs; etc.
   - `Tasks:` enumerated actionable items to update each dependent doc (e.g., "Update Observer events table with <EventName>").
3. Persist modifications to `/memory/constitution_update_checklist.md`.

## 8. Execute Checklist Propagation
1. Re-load `/memory/constitution_update_checklist.md`.
2. For each new `Tasks:` item just added, open and minimally update the referenced doc to reflect the constitutional change (add event definitions, builder option descriptions, deprecation notices, migration steps, etc.).
3. If any referenced doc is missing: `ERROR: Referenced dependent document missing <path>`.
4. After updates, mark each task with `- [x]` at end of line.
5. If any tasks remain unchecked: `ERROR: Unresolved checklist tasks`.

## 9. Final Validation
Ensure:
1. All error prefix formats exactly match `ERROR:` when present.
2. Scope related errors (if any) use only approved phrases:
   - `Module encroaches on core: <item>`
3. No dangling TODO / FIXME strings were introduced.
4. Roman numerals remain sequential (scan headings). If gap: `ERROR: Article numbering gap`.
5. Pattern Evolution Ledger lists all newly declared Builder options & Observer events.

## 10. Report Summary
Output a structured summary (Markdown acceptable) containing:
1. Counts: Added Articles, Amended Articles, Deprecated Articles, Clarifications, Glossary Terms Added.
2. Builder options added (name + default) & Observer events added (name + timing).
3. Any deprecations with effective versions.
4. List of dependent docs updated.
5. Confirmation that checklist tasks all checked.
6. Statement: `Constitution update complete.`

If any fatal issue encountered at any phase, emit only the first encountered `ERROR:` line (no partial writes to constitution or checklist) and abort without modifying files.

---
### Error Conditions (Authoritative List)
Use EXACT strings:
- `ERROR: Unparseable proposed change`
- `ERROR: Constitution format invalid`
- `ERROR: Duplicate article title <title>`
- `ERROR: Target article not found <target>`
- `ERROR: Invalid deprecation metadata <target>`
- `ERROR: Missing pattern evaluation for interface change <target or title>`
- `ERROR: Module encroaches on core: <description>`
- `ERROR: Missing pattern evaluation for API change: <symbol or article>`
- `ERROR: Referenced dependent document missing <path>`
- `ERROR: Unresolved checklist tasks`
- `ERROR: Article numbering gap`

Non-fatal warning example (emit but proceed): `WARN: Missing bounded context glossary (created)`.

---
### Implementation Notes
1. All file edits must be atomic: construct new content fully in memory then write once.
2. Preserve existing article text verbatim unless explicitly amended / deprecated.
3. Do not renumber existing Articles; only append new highest-number Article for NEW additions.
4. Maintain alphabetical order in Glossary.
5. Avoid trailing spaces; ensure single newline termination.
6. Prefer concise, imperative language in amendment notes.

---
### Success Criteria
- No `ERROR:` lines emitted.
- Constitution updated with all requested changes.
- Checklist updated, tasks executed and checked.
- Pattern Evolution Ledger reflects new pattern artifacts.
- Summary produced with required counts.
