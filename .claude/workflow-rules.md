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

## Workflow Enforcement
This rule applies to all future development work in this repository and should be followed consistently to maintain code quality standards.