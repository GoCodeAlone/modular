# Modular Framework Examples

This directory contains practical examples demonstrating various features and use cases of the modular framework. Each example is a complete, working application that showcases different aspects of building modular applications.

## 📚 Available Examples

### [Basic App](./basic-app/) - Foundation Example
**Demonstrates**: Core modular application concepts
- Basic application setup with `modular.NewStdApplication()`
- Configuration management with YAML and environment variables
- Custom module creation and registration
- Service dependencies between modules
- Configuration validation and defaults
- Application lifecycle management

**Best for**: Getting started with the modular framework

### [Reverse Proxy](./reverse-proxy/) - Networking Example
**Demonstrates**: HTTP reverse proxy with routing
- ChiMux router integration with CORS middleware
- Reverse proxy configuration and backend services
- HTTP server module usage
- Multi-module composition
- Graceful shutdown handling

**Best for**: Building API gateways and service proxies

### [HTTP Client](./http-client/) - Client Integration Example
**Demonstrates**: Advanced HTTP client configuration
- HTTP client module with connection pooling
- Integration with reverse proxy modules
- Performance tuning and timeout configuration
- Module service dependencies
- Basic HTTP client logging

**Best for**: High-performance HTTP client applications

### [Advanced Logging](./advanced-logging/) - Debugging Example
**Demonstrates**: Comprehensive HTTP client logging
- Detailed request/response logging
- File-based logging with organized structure
- Header and body logging capabilities
- Configurable logging levels and limits
- Real-world HTTP traffic logging

**Best for**: Debugging, monitoring, and compliance requirements

## 🚀 Getting Started

Each example is self-contained and can be run independently:

```bash
# Navigate to any example directory
cd examples/basic-app

# Build the example
go build .

# Run the example
./basic-app
```

## 🏗️ Example Structure

Each example follows a consistent structure:

```
example-name/
├── README.md           # Detailed documentation
├── go.mod             # Go module configuration
├── config.yaml        # Application configuration
├── main.go            # Main application file
└── [additional files] # Example-specific code
```

## 🧪 Validation

All examples are automatically validated through CI/CD to ensure they:
- ✅ Build successfully with `go build`
- ✅ Start without immediate errors
- ✅ Have proper module configuration
- ✅ Follow framework best practices

## 📖 Learning Path

**Recommended order for learning:**

1. **[Basic App](./basic-app/)** - Start here to understand core concepts
2. **[Reverse Proxy](./reverse-proxy/)** - Learn about networking modules
3. **[HTTP Client](./http-client/)** - Explore client-side functionality
4. **[Advanced Logging](./advanced-logging/)** - Master debugging and monitoring

## 🛠️ Building Your Own

Use these examples as templates for your own applications:

1. **Copy an example** that closely matches your use case
2. **Modify the configuration** in `config.yaml`
3. **Add your custom modules** following the patterns shown
4. **Update the dependencies** in `go.mod` as needed

## 🔧 Configuration

All examples use YAML configuration with support for:
- Environment variable overrides
- Default values
- Required field validation
- Type conversion and validation

Configuration files follow the pattern:
```yaml
# Module configurations
modulename:
  setting1: value1
  setting2: value2

# Application-specific settings
app:
  name: "My App"
  environment: "dev"
```

## 📋 Module Categories

Examples demonstrate these module categories:

| Category | Examples | Modules Used |
|----------|----------|--------------|
| **Web Servers** | basic-app, reverse-proxy | httpserver, chimux |
| **HTTP Clients** | http-client, advanced-logging | httpclient |
| **Routing & Middleware** | reverse-proxy, http-client | chimux, reverseproxy |
| **Custom Modules** | basic-app | webserver, router, api |

## 🎯 Common Patterns

The examples demonstrate these important patterns:

- **Module Registration Order**: Dependencies first, consumers second
- **Configuration Structure**: Module-specific sections in YAML
- **Service Dependencies**: How modules provide services to each other
- **Lifecycle Management**: Proper startup, running, and shutdown
- **Error Handling**: Graceful error handling and logging

## 🤝 Contributing

When adding new examples:

1. Follow the established directory structure
2. Include comprehensive README documentation
3. Add proper go.mod configuration with replace directives
4. Ensure the example builds and runs successfully
5. Update this main README with the new example

## 🔗 Related Documentation

- [Main Framework Documentation](../README.md)
- [Module Documentation](../modules/README.md)
- [API Reference](../docs/)

Each example includes detailed documentation and can serve as a reference for building your own modular applications.
