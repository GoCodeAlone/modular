# /copilot Command

## Purpose
Analyze available tasks and delegate appropriate ones to GitHub Copilot Coding Agent, creating parallel pull requests for items that can be clearly described and itemized.

## Usage
```
/copilot [task_description]
```

## Process
1. **Task Analysis**: Examine the current task list and identify items suitable for GitHub Copilot
2. **Parallelization**: Group tasks that can be worked on simultaneously 
3. **PR Creation**: Create separate PRs for each batch using GitHub MCP
4. **Tracking**: Store PR IDs in `.claude/pr-tracker.json`
5. **Remaining Work**: Delegate complex tasks to TDD developer agent

## Criteria for Copilot Delegation
- Clearly defined requirements
- Isolated functionality
- Well-described acceptance criteria
- No complex architectural decisions
- Suitable for automated implementation

## Example Tasks for Copilot
- Bug fixes with clear reproduction steps
- Feature additions with detailed specifications
- Test implementations for existing code
- Documentation updates
- Configuration enhancements
- Linter/formatting fixes

## Repository Configuration
- **Owner**: GoCodeAlone
- **Repo**: modular
- **Base Branch**: 001-baseline-specification-for (or current branch)