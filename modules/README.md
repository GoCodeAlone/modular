# Modular Modules

This directory contains a collection of reusable modules for the [Modular](https://github.com/GoCodeAlone/modular) framework. Each module is independently versioned and can be imported separately.

[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)

## Available Modules

| Module                     | Description                              | Configuration                           | Dependencies                           |
|----------------------------|------------------------------------------|-----------------------------------------|----------------------------------------|
| [chimux](./chimux)         | Chi router integration for Modular       | [Yes](./chimux/config.go)               | -                                      |
| [database](./database)     | Database connectivity with SQL operations | [Yes](./database/config.go)            | -                                      |
| [httpclient](./httpclient) | Configurable HTTP client with connection pooling, timeouts, and verbose logging | [Yes](./httpclient/config.go)          | -                                      |
| [httpserver](./httpserver) | HTTP/HTTPS server with TLS support, graceful shutdown, and configurable timeouts | [Yes](./httpserver/config.go)          | -                                      |
| [jsonschema](./jsonschema) | Provides JSON Schema validation services | No                                     | -                                      |
| [letsencrypt](./letsencrypt) | SSL/TLS certificate automation with Let's Encrypt | [Yes](./letsencrypt/config.go) | [httpserver](./httpserver)             |
| [reverseproxy](./reverseproxy) | Reverse proxy with routing capabilities | [Yes](./reverseproxy/config.go)        | -                                      |

## Using Modules

Each module can be imported and used independently:

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/jsonschema"
    "github.com/GoCodeAlone/modular/modules/database"
)

// Register the modules with your Modular application
app.RegisterModule(jsonschema.NewModule())
app.RegisterModule(database.NewModule())
```

## Module Structure

All modules in this directory follow a common structure:

- Implement the `modular.Module` interface
- Provide a `NewModule()` constructor function
- Include comprehensive tests
- Include a README.md with usage instructions

## Contributing New Modules

If you'd like to contribute a module:

1. Create a new directory in `modules/` with your module name
2. Implement the `modular.Module` interface
3. Include thorough tests and documentation
4. Submit a pull request

## License

All modules are licensed under the [MIT License](../LICENSE).
