# Modular Framework - Available Modules

This directory contains all the pre-built modules available in the Modular framework. Each module is designed to be plug-and-play, well-documented, and production-ready.

[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)

## 📋 Module Directory

| Module                     | Description                              | Configuration | Dependencies                           | Go Docs |
|----------------------------|------------------------------------------|---------------|----------------------------------------|---------|
| [auth](./auth)             | Authentication and authorization with JWT, sessions, password hashing, and OAuth2/OIDC support | [Yes](./auth/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/auth.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/auth) |
| [cache](./cache)           | Multi-backend caching with Redis and in-memory support | [Yes](./cache/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/cache.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/cache) |
| [chimux](./chimux)         | Chi router integration with middleware support | [Yes](./chimux/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/chimux.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/chimux) |
| [database](./database)     | Database connectivity and SQL operations with multiple driver support | [Yes](./database/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/database.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/database) |
| [eventbus](./eventbus)     | Asynchronous event handling and pub/sub messaging | [Yes](./eventbus/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/eventbus.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/eventbus) |
| [eventlogger](./eventlogger) | Structured logging for Observer pattern events with CloudEvents support | [Yes](./eventlogger/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/eventlogger.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/eventlogger) |
| [httpclient](./httpclient) | Configurable HTTP client with connection pooling, timeouts, and verbose logging | [Yes](./httpclient/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/httpclient.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/httpclient) |
| [httpserver](./httpserver) | HTTP/HTTPS server with TLS support, graceful shutdown, and configurable timeouts | [Yes](./httpserver/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/httpserver.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/httpserver) |
| [jsonschema](./jsonschema) | JSON Schema validation services | No | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/jsonschema.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/jsonschema) |
| [letsencrypt](./letsencrypt) | SSL/TLS certificate automation with Let's Encrypt | [Yes](./letsencrypt/config.go) | Works with httpserver | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/letsencrypt.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/letsencrypt) |
| [reverseproxy](./reverseproxy) | Reverse proxy with load balancing, circuit breaker, and health monitoring | [Yes](./reverseproxy/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/reverseproxy.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/reverseproxy) |
| [scheduler](./scheduler)   | Job scheduling with cron expressions and worker pools | [Yes](./scheduler/config.go) | - | [![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/scheduler.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/scheduler) |

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
