# HTTP Client Module

This module provides a configurable HTTP client service that can be used by other modules in the modular framework. It supports configurable connection pooling, timeouts, and optional verbose logging of HTTP requests and responses.

## Features

- Configurable HTTP client settings including connection pooling and timeouts
- Optional verbose logging of HTTP requests and responses
- Support for logging to files or application logger
- Request modifier support for customizing requests before they are sent
- Easy integration with other modules through service dependencies

## Configuration

The module can be configured using YAML, JSON, or environment variables:

```yaml
httpclient:
  max_idle_conns: 100             # Maximum idle connections across all hosts
  max_idle_conns_per_host: 10     # Maximum idle connections per host
  idle_conn_timeout: 90           # Maximum time an idle connection is kept alive (seconds)
  request_timeout: 30             # Default timeout for HTTP requests (seconds)
  tls_timeout: 10                 # TLS handshake timeout (seconds)
  disable_compression: false      # Whether to disable response body compression
  disable_keep_alives: false      # Whether to disable HTTP keep-alives
  verbose: false                  # Enable verbose logging of HTTP requests and responses
  verbose_options:                # Options for verbose logging (when verbose is true)
    log_headers: true             # Log request and response headers
    log_body: true                # Log request and response bodies
    max_body_log_size: 10000      # Maximum size of logged bodies (bytes)
    log_to_file: false            # Whether to log to files instead of application logger
    log_file_path: "/tmp/logs"    # Directory path for log files (required when log_to_file is true)
```

## Integration with Other Modules

The HTTP client module provides a `ClientService` that can be used by other modules through service dependency injection. For example, to use this client in the reverseproxy module:

```go
// In reverseproxy module:
func (m *ReverseProxyModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name: "router", 
			Required: true, 
			MatchByInterface: true, 
			SatisfiesInterface: reflect.TypeOf((*handleFuncService)(nil)).Elem(),
		},
		{
			Name: "httpclient",
			Required: false, // Optional dependency
			MatchByInterface: true,
			SatisfiesInterface: reflect.TypeOf((*httpclient.ClientService)(nil)).Elem(),
		},
	}
}
```

Then in the constructor:

```go
func (m *ReverseProxyModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get router service
		handleFuncSvc, ok := services["router"].(handleFuncService)
		if !ok {
			return nil, fmt.Errorf("service %s does not implement HandleFunc interface", "router")
		}
		m.router = handleFuncSvc

		// Get optional HTTP client service
		if clientService, ok := services["httpclient"].(httpclient.ClientService); ok {
			// Use the provided HTTP client
			m.httpClient = clientService.Client()
		} else {
			// Create a default HTTP client
			m.httpClient = &http.Client{
				// Default settings...
			}
		}

		return m, nil
	}
}
```

## Usage Example

```go
package main

import (
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/httpclient"
	"github.com/GoCodeAlone/modular/modules/reverseproxy"
)

func main() {
	app := modular.NewApplication()
	
	// Register modules
	app.RegisterModule(httpclient.NewHTTPClientModule())
	app.RegisterModule(reverseproxy.NewModule())
	
	// The reverseproxy module will automatically use the httpclient service if available
	
	// Run the application
	if err := app.Run(); err != nil {
		panic(err)
	}
}
```