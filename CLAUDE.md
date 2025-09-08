# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

### Core Framework
```bash
# Format code
go fmt ./...

# Run linter (required before commit)
golangci-lint run

# Run all core tests
go test ./... -v

# Run specific tests with race detection
go test -race ./... -v

# Run BDD tests in parallel (faster local feedback)
chmod +x scripts/run-module-bdd-parallel.sh
scripts/run-module-bdd-parallel.sh 6  # 6 workers, or omit for auto-detect
```

### Modules
```bash
# Test all modules
for module in modules/*/; do
  if [ -f "$module/go.mod" ]; then
    echo "Testing $module"
    cd "$module" && go test ./... -v && cd -
  fi
done

# Test specific module
cd modules/database && go test ./... -v
```

### Examples
```bash
# Test all examples
for example in examples/*/; do
  if [ -f "$example/go.mod" ]; then
    echo "Testing $example"
    cd "$example" && go test ./... -v && cd -
  fi
done

# Build and run specific example
cd examples/basic-app
GOWORK=off go build
./basic-app
```

### CLI Tool
```bash
# Test CLI
cd cmd/modcli && go test ./... -v

# Install CLI tool
go install github.com/GoCodeAlone/modular/cmd/modcli@latest

# Generate module or config
modcli generate module --name MyFeature
modcli generate config --name Server
```

## High-Level Architecture

### Core Framework Structure
The Modular framework implements a plugin-based architecture with dependency injection and lifecycle management:

- **Application** (`application.go`): Central orchestrator managing module lifecycle, dependency resolution, and service registry
- **Module Interface** (`module.go`): Core contract for all modules defining lifecycle hooks and service provision
- **Service Registry**: Dynamic registration and resolution supporting both named and interface-based matching
- **Configuration System**: Multi-source config loading with validation, defaults, and tenant awareness
- **Observer Pattern**: Event-driven communication with CloudEvents support for standardized event handling
- **Multi-tenancy**: Built-in tenant isolation with context propagation and tenant-aware configuration

### Module System
Modules follow a consistent lifecycle pattern:
1. **RegisterConfig**: Register configuration sections
2. **Init**: Initialize module, resolve dependencies
3. **Start**: Begin module operation
4. **Stop**: Graceful shutdown

Key module patterns:
- **Service Dependencies**: Modules declare required/optional services, framework injects them
- **Interface Matching**: Services can be matched by interface compatibility, not just name
- **Tenant Awareness**: Modules can implement `TenantAwareModule` for multi-tenant support
- **Constructor Injection**: Modules can use constructor pattern for dependency injection

### Configuration Management
- **Feeders**: Pluggable configuration sources (env, yaml, json, toml, dotenv)
- **Validation**: Struct tags for defaults, required fields, custom validators
- **Tenant-Aware**: Per-tenant configuration overrides with isolation
- **Dynamic Reload**: Supported for select fields with careful concurrency handling

### Concurrency & Safety
- **Race-Free**: All code passes `go test -race`
- **Observer Pattern**: RWMutex protection with snapshot-based notification
- **Defensive Copying**: Maps/slices copied on construction to prevent external mutation
- **Request Body Handling**: Pre-read for parallel fan-out scenarios
- **Synchronization**: Explicit mutex documentation for protected resources

## Development Guidelines

### Code Standards
- **Go 1.25+**: Uses latest Go features (toolchain 1.25.0)
- **Formatting**: Always run `go fmt ./...` before commits
- **Linting**: Must pass `golangci-lint run` (see `.golangci.yml`)
- **Testing**: Comprehensive unit, integration, and BDD tests required
- **Documentation**: GoDoc comments for all exported symbols

### Pattern Evolution
- **Builder/Options**: Add capabilities via option functions, never modify constructors
- **Observer Events**: Emit events for cross-cutting concerns vs interface widening
- **Interface Design**: Prefer new narrow interfaces over widening existing ones
- **Backwards Compatibility**: Maintain API compatibility with deprecation paths

### Module Development
- Implement core `Module` interface minimum
- Optional interfaces: `Startable`, `Stoppable`, `TenantAwareModule`
- Provide comprehensive configuration with validation
- Register services for other modules to consume
- Include README with examples and configuration reference

### Testing Strategy
- **BDD Tests**: Feature files with Cucumber/Godog for behavior specification
- **Unit Tests**: Isolated function/method testing
- **Integration Tests**: Module interaction and service dependency testing
- **Parallel Testing**: Use per-app config feeders, avoid global mutation
- **Race Detection**: All tests must pass with `-race` flag

### Key Files & Patterns
- **Project Constitution** (`memory/constitution.md`): Core principles and governance
- **Go Best Practices** (`GO_BEST_PRACTICES.md`): Actionable checklists and patterns
- **Concurrency Guidelines** (`CONCURRENCY_GUIDELINES.md`): Race avoidance patterns
- **GitHub Copilot Instructions** (`.github/copilot-instructions.md`): PR review guidance

## Common Development Tasks

### Adding a New Module
1. Create directory `modules/mymodule/`
2. Initialize go.mod: `cd modules/mymodule && go mod init github.com/GoCodeAlone/modular/modules/mymodule`
3. Implement `Module` interface in `module.go`
4. Add configuration struct with validation tags
5. Write comprehensive tests including BDD features
6. Create README with usage examples

### Debugging Module Issues
```go
// Debug specific module
modular.DebugModuleInterfaces(app, "module-name")

// Debug all modules
modular.DebugAllModuleInterfaces(app)
```

### Configuration Validation
```go
// Struct tags for validation
type Config struct {
    Host string `yaml:"host" default:"localhost" desc:"Server host"`
    Port int    `yaml:"port" default:"8080" required:"true" desc:"Port number"`
}

// Custom validation
func (c *Config) Validate() error {
    if c.Port < 1024 || c.Port > 65535 {
        return fmt.Errorf("port must be between 1024 and 65535")
    }
    return nil
}
```

### Generate Sample Config
```go
cfg := &AppConfig{}
err := modular.SaveSampleConfig(cfg, "yaml", "config-sample.yaml")
```

## Important Notes

- **No Global Mutation**: Tests use per-app config feeders via `app.SetConfigFeeders()`
- **Defensive Patterns**: Always copy external maps/slices in constructors
- **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` pattern
- **Logging Keys**: Standard fields: `module`, `tenant`, `instance`, `phase`, `event`
- **Performance**: Avoid reflection in hot paths, benchmark critical sections
- **Security**: Never log secrets, use proper error messages without exposing internals