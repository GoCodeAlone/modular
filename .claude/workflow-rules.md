# Claude Code Workflow Rules

This file defines automated workflow patterns for code development in this repository.

## Code Review Automation

### Rule: TDD-Developer → Go-DDD-Code-Reviewer Pipeline
**Trigger**: After `tdd-developer` agent completes any code implementation task
**Action**: Automatically invoke `go-ddd-code-reviewer` agent to review the implementation
**Scope**: All Go code changes, especially:
- New feature implementations
- Interface implementations
- Domain logic changes
- Module integrations
- Service implementations

### Implementation Pattern:
```
1. TDD-Developer Agent implements feature following TDD methodology
2. Code Review Agent automatically reviews implementation for:
   - Go best practices compliance
   - Domain-Driven Design principles
   - Test quality and real implementation validation
   - Race condition detection
   - Performance considerations
   - Error handling patterns
   - Documentation completeness

3. Any issues found are addressed before marking task complete
```

### Benefits:
- Ensures consistent code quality across all implementations
- Catches potential issues early in development cycle
- Maintains adherence to project standards and patterns
- Provides continuous learning feedback for development practices

## Review Checklist Integration
The code reviewer should validate against:
- **CLAUDE.md**: Development patterns and guidelines
- **GO_BEST_PRACTICES.md**: Go-specific implementation standards  
- **CONCURRENCY_GUIDELINES.md**: Race-free patterns
- **Project Constitution**: Core principles and governance
- **Design Brief Compliance**: Feature specification adherence

## GitHub Copilot Integration

### /copilot Command
**Purpose**: Delegate clearly defined tasks to GitHub Copilot Coding Agent for parallel development

**Usage**: `/copilot [task_description]`

**Process**:
1. Analyze current tasks and identify suitable candidates for Copilot
2. Create separate PRs for parallel work streams
3. Track PR IDs in `.claude/pr-tracker.json`
4. Use TDD developer agent for complex architectural work

**Suitable Tasks for Copilot**:
- Bug fixes with clear reproduction steps
- Feature additions with detailed specifications
- Test implementations for existing code
- Documentation updates
- Configuration enhancements
- Isolated functionality improvements

**Implementation**:
```markdown
When /copilot command is used:
1. Confirm repository details (GoCodeAlone/modular)
2. Use current branch (001-baseline-specification-for) as base_ref
3. Create detailed problem_statement for Copilot
4. Submit via GitHub MCP: create_pull_request_with_copilot
5. Track PR ID in memory system
6. Delegate remaining complex tasks to TDD agent
```

### /review-prs Command
**Purpose**: Automated review and management of GitHub Copilot PRs

**Usage**: `/review-prs`

**Process**:
1. Load active PRs from `.claude/pr-tracker.json`
2. Use go-ddd-code-reviewer agent for comprehensive review
3. Check CI status (tests, linter, security)
4. Verify implementation matches stated goals
5. Post review comments with @copilot tag for issues
6. Auto-merge compliant PRs and sync locally
7. Update tracker with completed PRs

**Review Criteria**:
- ✅ All CI checks pass (tests, linter, formatting)
- ✅ No placeholder or TODO logic
- ✅ Implementation matches PR objectives
- ✅ Follows Go best practices and DDD principles
- ✅ Proper error handling and race-free patterns
- ✅ Adequate test coverage

**Auto-Review Schedule**: 
- Manual execution or automated every 15 minutes
- Triggered after PR creation
- Continues until PR is merged or closed

## Workflow Enforcement
These rules apply to all future development work in this repository and should be followed consistently to maintain code quality standards and enable efficient parallel development.