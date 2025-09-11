# HTTP Server Module

[![Go Reference](https://pkg.go.dev/badge/github.com/CrisisTextLine/modular/modules/httpserver.svg)](https://pkg.go.dev/github.com/CrisisTextLine/modular/modules/httpserver)

This module provides HTTP/HTTPS server capabilities for the modular framework. It handles listening on a specified port, TLS configuration, and server timeouts.

## Features

- HTTP and HTTPS support
- Configurable host and port bindings
- Customizable timeouts
- Graceful shutdown
- TLS support

## Configuration

The module can be configured using YAML, JSON, or environment variables:

```yaml
httpserver:
  host: "0.0.0.0"     # Host to bind to (default: 0.0.0.0)
  port: 8080          # Port to listen on (default: 8080)
  read_timeout: 15    # Maximum duration for reading requests (seconds)
  write_timeout: 15   # Maximum duration for writing responses (seconds)
  idle_timeout: 60    # Maximum time to wait for the next request (seconds)
  shutdown_timeout: 30 # Maximum time for graceful shutdown (seconds)
  tls:
    enabled: false    # Whether TLS is enabled
    cert_file: ""     # Path to TLS certificate file
    key_file: ""      # Path to TLS private key file
```

## Usage

This module works with other modules in the application:

1. It depends on a router module (like `chimux`) which provides the HTTP handler.
2. The HTTP server listens on the configured port and passes incoming requests to the router.
3. The router then directs requests to appropriate handlers, such as the `reverseproxy` module.

### Integration Example

```go
package main

import (
	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/CrisisTextLine/modular/modules/httpserver"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
)

func main() {
	app := modular.NewApplication()
	
	// Register modules in the appropriate order
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(reverseproxy.NewReverseProxyModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())
	
	// Initialize and run the application
	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

## Architecture

The HTTP server module integrates with the modular framework as follows:

```
┌────────────────────┐     ┌────────────────────┐     ┌────────────────────┐
│                    │     │                    │     │                    │
│    HTTP Server     │────▶│    Router (Chi)    │────▶│   Reverse Proxy    │
│                    │     │                    │     │                    │
└────────────────────┘     └────────────────────┘     └────────────────────┘
   Listens on port         Routes based on URL        Proxies to backends
```

## Dependencies

- Requires a module that provides the `router` service implementing `http.Handler`
- Typically used with the `chimux` module