# Modular Framework - Available Modules

This directory contains all the pre-built modules available in the Modular framework. Each module is designed to be plug-and-play, well-documented, and production-ready.

[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)

## 📋 Module Directory

| Module                     | Description                              | Configuration                           | Dependencies                           |
|----------------------------|------------------------------------------|-----------------------------------------|----------------------------------------|
| [chimux](./chimux)         | Chi router integration for Modular       | [Yes](./chimux/config.go)               | -                                      |
| [database](./database)     | Database connectivity with SQL operations | [Yes](./database/config.go)            | -                                      |
| [httpclient](./httpclient) | Configurable HTTP client with connection pooling, timeouts, and verbose logging | [Yes](./httpclient/config.go)          | -                                      |
| [httpserver](./httpserver) | HTTP/HTTPS server with TLS support, graceful shutdown, and configurable timeouts | [Yes](./httpserver/config.go)          | -                                      |
| [jsonschema](./jsonschema) | Provides JSON Schema validation services | No                                     | -                                      |
| [letsencrypt](./letsencrypt) | SSL/TLS certificate automation with Let's Encrypt | [Yes](./letsencrypt/config.go) | [httpserver](./httpserver)             |
| [reverseproxy](./reverseproxy) | Reverse proxy with routing capabilities | [Yes](./reverseproxy/config.go)        | -                                      |

### Core Modules
- **[Auth](auth/README.md)** - Authentication and authorization with JWT, sessions, password hashing, and OAuth2/OIDC support
- **[Cache](cache/README.md)** - Multi-backend caching solution with Redis and in-memory implementations
- **[Database](database/README.md)** - Database connectivity and management with support for multiple drivers
- **[Event Bus](eventbus/README.md)** - Asynchronous event handling and pub/sub messaging system

### Network & Communication
- **[Chi Router (Chimux)](chimux/README.md)** - HTTP routing with Chi router integration and comprehensive middleware support
- **[Reverse Proxy](reverseproxy/README.md)** - Advanced reverse proxy with load balancing, circuit breaker, and health monitoring

### Utilities & Processing
- **[JSON Schema](jsonschema/README.md)** - JSON schema validation and data processing capabilities
- **[Scheduler](scheduler/README.md)** - Job scheduling system with cron expressions and worker pool management

## 🚀 Quick Start

Each module follows the same integration pattern:

1. **Install the module** (if using as separate dependency)
2. **Configure** via YAML, environment variables, or programmatically
3. **Register** with your modular application
4. **Use** the module's services in your application

## 📖 Module Structure

Every module includes:
- **README.md** - Complete documentation with examples
- **config.go** - Configuration structures and validation
- **module.go** - Module implementation and service registration
- **service.go** - Core service implementation
- **Tests** - Comprehensive test coverage

## 🔧 Configuration

All modules support multiple configuration sources:
- YAML configuration files
- Environment variables
- Programmatic configuration
- Tenant-specific overrides

## 🤝 Contributing

When creating new modules, please follow the established patterns and ensure:
- Comprehensive documentation
- Full test coverage
- Configuration validation
- Error handling best practices
- Consistent API design

For more information, see the main [project documentation](../README.md).
