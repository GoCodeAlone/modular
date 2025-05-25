# Modular Modules

This directory contains a collection of reusable modules for the [Modular](https://github.com/GoCodeAlone/modular) framework. Each module is independently versioned and can be imported separately.

[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)

## Available Modules

| Module                     | Description                              | Documentation                           |
|----------------------------|------------------------------------------|-----------------------------------------|
| [cache](./cache)           | Caching service with multiple backends   | [Documentation](./cache/README.md)      |
| [chimux](./chimux)         | Chi router integration for Modular       | [Documentation](./chimux/README.md)     |
| [database](./database)     | Database connectivity and SQL operations | [Documentation](./database/README.md)   |
| [eventbus](./eventbus)     | Publish-subscribe messaging system       | [Documentation](./eventbus/README.md)   |
| [jsonschema](./jsonschema) | Provides JSON Schema validation services | [Documentation](./jsonschema/README.md) |
| [reverseproxy](./reverseproxy) | Reverse proxy with routing capabilities | [Documentation](./reverseproxy/README.md) |
| [scheduler](./scheduler)   | Job scheduling with cron support         | [Documentation](./scheduler/README.md)  |

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
