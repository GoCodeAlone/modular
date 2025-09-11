# Claude Code Project Prompts

This directory contains Claude-specific prompts and instructions for working with the Modular framework.

## Available Prompts

### Development Workflow Prompts
Located in `.github/prompts/`:
- `specify.prompt.md` - Start new feature with specification (Step 1)
- `plan.prompt.md` - Create implementation plan (Step 2)  
- `tasks.prompt.md` - Break down plan into tasks (Step 3)
- `constitution.prompt.md` - Project governance reference

### Usage Examples

```bash
# Start a new feature
"Please follow .github/prompts/specify.prompt.md to create a feature for adding rate limiting"

# Break down existing plan
"Please follow .github/prompts/tasks.prompt.md for the current feature"

# Create implementation plan
"Please follow .github/prompts/plan.prompt.md for the specification in features/001-rate-limiting/"
```

## Custom Prompts

You can add your own prompts to `.claude/prompts/`:

1. Create a new `.md` file in `.claude/prompts/`
2. Include clear instructions and context requirements
3. Reference it by path when talking to Claude

## Project Standards

When using these prompts, Claude will automatically:
- Follow TDD practices (tests before implementation)
- Classify functionality as CORE or MODULE
- Apply Builder/Observer patterns for API evolution
- Ensure race-free concurrency patterns
- Generate appropriate documentation

## Tips for Effective Prompt Usage

1. **Provide Context**: Always include the feature name or description
2. **Specify Scope**: Indicate if working on CORE or specific MODULE
3. **Reference Files**: Point to existing specs, plans, or code
4. **Chain Prompts**: Use specify → plan → tasks workflow for new features

## Integration with Project Scripts

These prompts integrate with project automation:
- `scripts/create-new-feature.sh` - Creates feature branches
- `scripts/check-task-prerequisites.sh` - Validates task prerequisites
- `scripts/run-module-bdd-parallel.sh` - Runs BDD tests in parallel

## See Also

- `CLAUDE.md` - Main Claude Code guidance file
- `GO_BEST_PRACTICES.md` - Go development standards
- `CONCURRENCY_GUIDELINES.md` - Race-free patterns
- `.github/copilot-instructions.md` - GitHub Copilot settings