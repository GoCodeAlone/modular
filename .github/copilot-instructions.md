This is the Modular Go framework - a structured way to create modular applications in Go with module lifecycle management, dependency injection, configuration management, and multi-tenancy support. Please follow these guidelines when contributing:

## Code Standards

### Go Development
- **Formatting**: All Go code must be formatted with `gofmt`. Run `go fmt ./...` before committing
- **Linting**: Use `golangci-lint run` to check code quality (see `.golangci.yml` for configuration)
- **Testing**: 
  - Core framework tests: `go test ./... -v`
  - Module tests: Run tests for each module individually:
    ```bash
    for module in modules/*/; do
      if [ -f "$module/go.mod" ]; then
        echo "Testing $module"
        cd "$module" && go test ./... -v && cd -
      fi
    done
    ```
  - Example tests: Run tests for each example individually:
    ```bash
    for example in examples/*/; do
      if [ -f "$example/go.mod" ]; then
        echo "Testing $example"
        cd "$example" && go test ./... -v && cd -
      fi
    done
    ```
  - CLI tests: `cd cmd/modcli && go test ./... -v`
- **Module Development**: Follow the established module interface patterns and provide comprehensive configuration options

### Required Before Each Commit
- Format Go code with `gofmt`
- Run `golangci-lint run` and fix any linting issues
- Ensure all tests pass (core, modules, examples, and CLI):
  - Core: `go test ./... -v`
  - Modules: `for module in modules/*/; do [ -f "$module/go.mod" ] && (cd "$module" && go test ./... -v); done`
  - Examples: `for example in examples/*/; do [ -f "$example/go.mod" ] && (cd "$example" && go test ./... -v); done`
  - CLI: `cd cmd/modcli && go test ./... -v`
- Update documentation when adding new features or changing APIs
- Update module README files when modifying modules

## Development Workflow

### Local Development Setup
1. Clone the repository: `git clone https://github.com/CrisisTextLine/modular.git`
2. Install Go 1.23.0 or later (toolchain uses 1.24.2)
3. Install golangci-lint: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
4. Run tests to verify setup: `go test ./... -v`

### Working with Modules
- Each module is in `modules/<module-name>/` with its own `go.mod`
- Modules should implement the core `Module` interface
- Modules can optionally implement `Startable`, `Stoppable`, `TenantAwareModule`, etc.
- All modules should have comprehensive README documentation with examples

### Working with Examples
- Examples are in `examples/<example-name>/` with their own `go.mod` 
- Examples demonstrate real-world usage patterns
- Each example should be runnable with clear instructions
- Examples help validate that the framework works as intended

## Repository Structure

### Core Framework (`/`)
- **Root**: Core framework code including `application.go`, `module.go`, service registry, and configuration system
- **`feeders/`**: Configuration feeders for various sources (env, yaml, json, toml)
- **`cmd/modcli/`**: Command-line tool for generating modules and configurations

### Modules (`/modules`)
Available pre-built modules:
- **`auth/`**: Authentication and authorization with JWT, sessions, password hashing, OAuth2/OIDC
- **`cache/`**: Multi-backend caching with Redis and in-memory support
- **`chimux/`**: Chi router integration with middleware support
- **`database/`**: Database connectivity and SQL operations with multiple drivers
- **`eventbus/`**: Asynchronous event handling and pub/sub messaging
- **`httpclient/`**: Configurable HTTP client with connection pooling and timeouts
- **`httpserver/`**: HTTP/HTTPS server with TLS support and graceful shutdown
- **`jsonschema/`**: JSON Schema validation services
- **`letsencrypt/`**: SSL/TLS certificate automation with Let's Encrypt
- **`reverseproxy/`**: Reverse proxy with load balancing and circuit breaker
- **`scheduler/`**: Job scheduling with cron expressions and worker pools

### Examples (`/examples`)
Working example applications:
- **`basic-app/`**: Simple modular application with HTTP server and routing
- **`reverse-proxy/`**: HTTP reverse proxy server with load balancing
- **`http-client/`**: HTTP client with proxy backend integration
- **`advanced-logging/`**: Advanced HTTP client logging and debugging
- **`instance-aware-db/`**: Database configuration with instance awareness
- **`multi-tenant-app/`**: Multi-tenant application example

## Key Guidelines

### Core Framework Development
1. **Module Interface Compliance**: Ensure all modules properly implement required interfaces
2. **Dependency Resolution**: Support both named and interface-based service matching
3. **Configuration System**: Support validation, defaults, required fields, and multiple formats
4. **Multi-tenancy**: Maintain tenant isolation and proper context handling
5. **Error Handling**: Use wrapped errors with clear messages and proper error types
6. **Backwards Compatibility**: Maintain API compatibility when possible

### Module Development
1. **Interface Implementation**: Implement core `Module` interface and relevant optional interfaces
2. **Configuration**: Provide comprehensive configuration with validation and defaults
3. **Service Provision**: Register services that other modules can depend on
4. **Documentation**: Include complete README with usage examples and configuration reference
5. **Testing**: Write comprehensive unit tests and integration tests where applicable
6. **Dependencies**: Minimize external dependencies and document any that are required

### Example Development
1. **Standalone Applications**: Each example should be a complete, runnable application
2. **Clear Documentation**: Include README with setup instructions and usage examples
3. **Real-world Patterns**: Demonstrate practical usage patterns and best practices
4. **Configuration**: Show different configuration approaches and validation
5. **Error Handling**: Demonstrate proper error handling and logging

### CLI Tool Development
1. **Code Generation**: Generate boilerplate code following established patterns
2. **Interactive Prompts**: Provide user-friendly interactive configuration
3. **Template System**: Use templates that reflect current best practices
4. **Validation**: Validate generated code and provide helpful error messages

### Configuration Best Practices
1. **Struct Tags**: Use `yaml`, `json`, `default`, `required`, and `desc` tags appropriately
2. **Validation**: Implement `ConfigValidator` interface for custom validation logic
3. **Documentation**: Use `desc` tags to document configuration options
4. **Defaults**: Provide sensible defaults for optional configuration
5. **Multiple Formats**: Support YAML, JSON, and TOML configuration formats

### Testing Strategy
1. **Unit Tests**: Test individual functions and methods in isolation
2. **Integration Tests**: Test module interactions and service dependencies
3. **Example Tests**: Ensure examples build and run correctly
4. **Mock Application**: Use the provided mock application for testing modules
5. **Interface Testing**: Verify modules implement interfaces correctly

### Multi-tenancy Guidelines
1. **Context Propagation**: Always propagate tenant context through the call chain
2. **Configuration Isolation**: Ensure tenant configurations are properly isolated
3. **Resource Management**: Handle tenant-specific resource creation and cleanup
4. **Service Isolation**: Maintain separation between tenant-specific services

### Error Handling Standards
1. **Error Wrapping**: Use `fmt.Errorf` with `%w` verb to wrap errors
2. **Error Types**: Define specific error types for different failure modes
3. **Context**: Include relevant context in error messages
4. **Logging**: Log errors at appropriate levels with structured logging
5. **Graceful Degradation**: Handle optional dependencies gracefully

## Development Tools

### CLI Tool (`modcli`)
- Generate new modules: `modcli generate module --name MyModule`
- Generate configurations: `modcli generate config --name MyConfig`
- Install with: `go install github.com/CrisisTextLine/modular/cmd/modcli@latest`

### Debugging Tools
- Debug module interfaces: `modular.DebugModuleInterfaces(app, "module-name")`
- Debug all modules: `modular.DebugAllModuleInterfaces(app)`
- Verbose logging for troubleshooting service dependencies and configuration

### Configuration Tools
- Generate sample configs: `modular.SaveSampleConfig(cfg, "yaml", "config-sample.yaml")`
- Support for YAML, JSON, and TOML formats
- Automatic validation and default value application