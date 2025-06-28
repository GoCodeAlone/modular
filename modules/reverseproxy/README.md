# Reverse Proxy Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/reverseproxy.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/reverseproxy)

A module for the [Modular](https://github.com/GoCodeAlone/modular) framework that provides a flexible reverse proxy with advanced routing capabilities.

## Overview

The Reverse Proxy module functions as a versatile API gateway that can route requests to multiple backend services, combine responses, and support tenant-specific routing configurations. It's designed to be flexible, extensible, and easily configurable.

## Key Features

* **Multi-Backend Routing**: Route HTTP requests to any number of configurable backend services
* **Response Aggregation**: Combine responses from multiple backends using various strategies
* **Custom Response Transformers**: Create custom functions to transform and merge backend responses
* **Tenant Awareness**: Support for multi-tenant environments with tenant-specific routing
* **Pattern-Based Routing**: Direct requests to specific backends based on URL patterns
* **Custom Endpoint Mapping**: Define flexible mappings from frontend endpoints to backend services

## Installation

```go
go get github.com/GoCodeAlone/modular/modules/reverseproxy@v1.0.0
```

## Usage

```go
package main

import (
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/reverseproxy"
	"log/slog"
	"os"
)

func main() {
	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
	)

	// Register required modules
	app.RegisterModule(chimux.NewChiMuxModule())
	
	// Register the reverseproxy module
	proxyModule, err := reverseproxy.NewModule()
	if err != nil {
		app.Logger().Error("Failed to create reverseproxy module", "error", err)
		os.Exit(1)
	}
	app.RegisterModule(proxyModule)

	// Run the application
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}
```

## Configuration

### Basic Configuration

```yaml
# config.yaml
reverseproxy:
  # Define your backend services
  backend_services:
    api: "http://api.example.com"
    auth: "http://auth.example.com"
    user: "http://user-service.example.com"
  
  # Set the default backend
  default_backend: "api"
  
  # Tenant-specific configuration
  tenant_id_header: "X-Tenant-ID"
  require_tenant_id: false
  
  # Composite routes for response aggregation
  composite_routes:
    "/api/user/profile":
      pattern: "/api/user/profile"
      backends: ["user", "api"]
      strategy: "merge"
```

### Advanced Features

The module supports several advanced features:

1. **Custom Response Transformers**: Create custom functions to transform responses from multiple backends
2. **Custom Endpoint Mappings**: Define detailed mappings between frontend endpoints and backend services
3. **Tenant-Specific Routing**: Route requests to different backend URLs based on tenant ID

For detailed documentation and examples, see the [DOCUMENTATION.md](DOCUMENTATION.md) file.

## License

[MIT License](LICENSE)
