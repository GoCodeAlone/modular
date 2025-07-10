# Reverse Proxy Module Documentation

## Table of Contents

1. [Introduction](#introduction)
2. [Architecture](#architecture)
3. [Installation](#installation)
4. [Configuration](#configuration)
   - [Configuration File Options](#configuration-file-options)
   - [CompositeRoute Configuration](#compositeroute-configuration)
   - [Circuit Breaker Configuration](#circuit-breaker-configuration)
5. [Basic Usage](#basic-usage)
   - [Simple Backend Routing](#simple-backend-routing)
   - [Default Routing](#default-routing)
6. [Advanced Usage](#advanced-usage)
   - [Composite Routes](#composite-routes)
   - [Response Transformation](#response-transformation)
   - [Custom Endpoint Mapping](#custom-endpoint-mapping)
   - [Tenant-Specific Routing](#tenant-specific-routing)
   - [Header Forwarding and Modification](#header-forwarding-and-modification)
   - [Error Handling](#error-handling)
   - [Timeout Management](#timeout-management)
   - [Redirect Handling](#redirect-handling)
7. [API Reference](#api-reference)
   - [Module Methods](#module-methods)
   - [Handler Types](#handler-types)
   - [Response Combiners](#response-combiners)
   - [Response Transformers](#response-transformers)
8. [Examples](#examples)
   - [API Gateway Example](#api-gateway-example)
   - [Response Aggregation Example](#response-aggregation-example)
   - [Custom Transformation Example](#custom-transformation-example)
   - [Error Handling Example](#error-handling-example)
   - [Partial Response Example](#partial-response-example)
9. [Performance Considerations](#performance-considerations)
10. [Security Best Practices](#security-best-practices)
11. [Troubleshooting](#troubleshooting)
12. [FAQ](#faq)

## Introduction

The Reverse Proxy module is a powerful and flexible API gateway component that routes HTTP requests to multiple backend services and provides advanced features for response aggregation, custom transformations, and tenant-aware routing. It's built for the [Modular](https://github.com/CrisisTextLine/modular) framework and designed to be easily configurable while supporting complex routing scenarios.

### Key Features

* **Multi-Backend Routing**: Route HTTP requests to different backend services based on URL patterns
* **Response Aggregation**: Combine responses from multiple services into a unified response
* **Custom Response Transformers**: Create custom functions to transform and combine backend responses
* **Tenant Awareness**: Support for multi-tenant environments with tenant-specific routing
* **Header Management**: Forward, add, or modify headers when routing to backend services
* **Error Handling**: Graceful handling of errors from backend services
* **Timeout Management**: Handle timeouts with configurable strategies
* **Redirect Support**: Smart handling of redirects from backend services
* **Flexible Configuration**: Multiple options for configuring routing behavior

## Architecture

The Reverse Proxy module consists of the following main components:

1. **ReverseProxyModule**: The main module implementation that initializes routing, handles requests, and manages configuration
2. **CompositeHandler**: A handler for routes that combine responses from multiple backend services
3. **BackendEndpointRequest**: Defines requests to be made to backend service endpoints
4. **EndpointMapping**: Maps frontend endpoints to backend endpoints with response transformation
5. **Response Combiners**: Functions that combine responses from multiple backends into a single response
6. **Response Transformers**: Custom functions that transform backend responses into the desired format

The module works by registering HTTP handlers with the router for specified patterns. When a request comes in, it determines which backend service(s) to route the request to, executes the request(s), and returns the response directly or combines multiple responses according to the configured strategy.

## Installation

To use the Reverse Proxy module in your Go application:

```go
go get github.com/CrisisTextLine/modular/modules/reverseproxy@v1.0.0
```

## Configuration

### Configuration File Options

The Reverse Proxy module uses a YAML configuration file with the following structure:

```yaml
reverseproxy:
  # Map of backend service identifiers to their base URLs
  backend_services:
    api: "http://api.example.com"
    auth: "http://auth.example.com"
    data: "http://data-service.example.com"
  
  # The default backend to route to if no specific route is matched
  default_backend: "api"
  
  # URL for an optional feature flag service
  feature_flag_service_url: "http://featureflags.example.com"
  
  # Header to use for tenant ID in multi-tenant environments
  tenant_id_header: "X-Tenant-ID"
  
  # Whether to require a tenant ID for requests
  require_tenant_id: false
  
  # Configuration for routes that combine responses from multiple backends
  composite_routes:
    "/api/users/{id}":
      pattern: "/api/users/{id}"
      backends: ["api", "data"]
      strategy: "merge"
  
  # Global timeout for backend requests (in seconds)
  timeout: 10
  
  # Headers to forward to backend services by default
  forward_headers:
    - "Authorization"
    - "User-Agent"
    - "X-Request-ID"
    
  # Global circuit breaker configuration
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    reset_timeout_seconds: 30
    
  # Per-backend circuit breaker configuration
  backend_circuit_breakers:
    api:
      enabled: true
      failure_threshold: 3  # more sensitive threshold for API backend
      reset_timeout_seconds: 15
    auth:
      enabled: false  # disable circuit breaker for auth backend
```

#### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| backend_services | map[string]string | Yes | - | Map of backend service identifiers to their base URLs |
| default_backend | string | No | First defined backend | The identifier of the default backend service to route to |
| feature_flag_service_url | string | No | - | URL for an optional feature flag service |
| tenant_id_header | string | No | X-Tenant-ID | Header to use for tenant ID in multi-tenant environments |
| require_tenant_id | bool | No | false | Whether to require a tenant ID for requests |
| composite_routes | map[string]CompositeRoute | No | - | Configuration for routes that combine responses from multiple backends |
| timeout | int | No | 30 | Global timeout for backend requests in seconds |
| forward_headers | []string | No | [] | List of headers to forward to backend services by default |
| circuit_breaker | CircuitBreakerConfig | No | See default values | Global circuit breaker configuration for all backends |
| backend_circuit_breakers | map[string]CircuitBreakerConfig | No | - | Per-backend circuit breaker configurations |

### CompositeRoute Configuration

A CompositeRoute defines how responses from multiple backends are combined for a specific route:

```yaml
composite_routes:
  "/api/example":
    # The URL pattern to match (using the router's syntax)
    pattern: "/api/example"
    # List of backend identifiers to route to
    backends: ["api", "data"]
    # Strategy for combining responses
    strategy: "merge"
    # Timeout specific to this route (in seconds)
    timeout: 5
    # Headers to add or override for this route
    headers:
      X-Route-Type: "composite"
```

#### Strategy Options

- `merge`: Merges JSON responses from all backends at the top level
- `select`: Returns the response from the first successful backend
- `append`: Appends array responses from all backends
- Custom strategies can be implemented programmatically

### Circuit Breaker Configuration

The circuit breaker pattern helps prevent cascading failures in distributed systems by temporarily disabling requests to failing backend services.

#### CircuitBreakerConfig Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| enabled | bool | No | true | Whether the circuit breaker is active |
| failure_threshold | int | No | 5 | Number of failures before opening the circuit |
| reset_timeout_seconds | int | No | 30 | Seconds to wait before trying a request when the circuit is open |

## Basic Usage

### Simple Backend Routing

To route all requests to a single backend service:

```yaml
reverseproxy:
  backend_services:
    api: "http://api.example.com"
  default_backend: "api"
```

This configuration forwards all requests to `http://api.example.com` with the same path and query parameters.

### Default Routing

The module can be configured to route to different backends based on path patterns:

```yaml
reverseproxy:
  backend_services:
    api: "http://api.example.com"
    auth: "http://auth.example.com"
  default_backend: "api"
```

In this configuration, requests will be routed to the `api` backend by default, unless overridden by more specific routing rules.

## Advanced Usage

### Composite Routes

Composite routes combine responses from multiple backend services:

```yaml
reverseproxy:
  backend_services:
    users: "http://users.example.com"
    preferences: "http://preferences.example.com"
  composite_routes:
    "/api/user-profile/{id}":
      pattern: "/api/user-profile/{id}"
      backends: ["users", "preferences"]
      strategy: "merge"
```

With this configuration, when a client requests `/api/user-profile/123`:

1. The proxy will make requests to both `http://users.example.com/api/user-profile/123` and `http://preferences.example.com/api/user-profile/123`
2. Assuming both return JSON responses, it will merge them at the top level
3. The combined response will be returned to the client

### Response Transformation

For more advanced response transformation, you can programmatically register custom response transformers:

```go
func main() {
    // Create and register the reverseproxy module
    proxyModule, _ := reverseproxy.NewModule()
    app.RegisterModule(proxyModule)
    
    // After the application is initialized...
    app.OnInit(func() {
        // Get the reverseproxy module instance
        module := app.GetModule("reverseproxy").(*reverseproxy.ReverseProxyModule)
        
        // Create a custom transformer
        customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
            // Process responses and create a custom combined response
            // ...
            
            return &reverseproxy.CompositeResponse{
                StatusCode: http.StatusOK,
                Headers:    headers,
                Body:       combinedBody,
            }, nil
        }
        
        // Register a custom endpoint with the transformer
        module.RegisterCustomEndpoint("/api/combined-data", reverseproxy.EndpointMapping{
            Endpoints: []reverseproxy.BackendEndpointRequest{
                {
                    Backend: "service1",
                    Path:    "/api/data",
                    Method:  "GET",
                },
                {
                    Backend: "service2",
                    Path:    "/api/related-data",
                    Method:  "GET",
                },
            },
            ResponseTransformer: customTransformer,
        })
    })
    
    // Run the application
    app.Run()
}
```

### Custom Endpoint Mapping

You can create detailed mappings between frontend endpoints and backend services:

```go
// Create a mapping that calls different endpoints on different backends
mapping := reverseproxy.EndpointMapping{
    Endpoints: []reverseproxy.BackendEndpointRequest{
        {
            Backend: "users",
            Path:    "/api/users/{id}",
            Method:  "GET",
        },
        {
            Backend: "orders",
            Path:    "/api/orders/by-user/{id}",
            Method:  "GET",
            QueryParams: map[string]string{
                "limit": "10",
            },
        },
    },
    ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
        // Custom transformation logic
        // ...
    },
}

// Register the custom endpoint
module.RegisterCustomEndpoint("/api/user-dashboard/{id}", mapping)
```

### Tenant-Specific Routing

The module supports tenant-specific routing for multi-tenant environments:

```yaml
reverseproxy:
  backend_services:
    api: "http://api.example.com"
    auth: "http://auth.example.com"
  tenant_id_header: "X-Tenant-ID"
  require_tenant_id: false
```

With this configuration:

1. The module checks for a `X-Tenant-ID` header in incoming requests
2. If present, it looks for tenant-specific configuration for the identified tenant
3. If tenant-specific routes are configured, it routes requests accordingly
4. If no tenant-specific route is found, it falls back to the default routes

Tenant-specific configuration is registered at runtime:

```go
func main() {
    // Create and register modules
    app.RegisterModule(proxyModule)
    
    // Register a tenant with specific backend URLs
    app.RegisterTenant("tenant1", map[string]modular.ConfigProvider{
        "reverseproxy": modular.NewStdConfigProvider(&reverseproxy.ReverseProxyConfig{
            BackendServices: map[string]string{
                "api": "http://tenant1-api.example.com",
                "auth": "http://tenant1-auth.example.com",
            },
        }),
    })
    
    // Run the application
    app.Run()
}
```

### Header Forwarding and Modification

You can control which headers are forwarded to backend services and add or modify headers:

```go
mapping := reverseproxy.EndpointMapping{
    Endpoints: []reverseproxy.BackendEndpointRequest{
        {
            Backend: "api",
            Path:    "/api/data",
            Method:  "GET",
            Headers: map[string]string{
                "X-Custom-Header": "custom-value",
                "X-Request-Source": "gateway",
            },
        },
    },
    ResponseTransformer: customTransformer,
}

module.RegisterCustomEndpoint("/api/gateway-data", mapping)
```

In this example:
- The `X-Custom-Header` with value "custom-value" will be added to the request
- The `X-Request-Source` header will be set to "gateway"
- Other headers from the original request will be forwarded according to the default configuration

### Error Handling

The module provides several options for handling errors from backend services:

1. **Default behavior**: When a backend service returns an error, the error response is passed to the response transformer, which can decide how to handle it.

2. **Custom error handling in transformers**:

```go
customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
    // Check for errors in responses
    if responses["service1"].StatusCode >= 400 {
        // Handle error from service1
        return &reverseproxy.CompositeResponse{
            StatusCode: http.StatusBadGateway,
            Headers:    http.Header{"Content-Type": []string{"application/json"}},
            Body:       []byte(`{"error":"Backend service error","details":"Service 1 is currently unavailable"}`),
        }, nil
    }
    
    // Process successful responses
    // ...
}
```

3. **Partial response handling**: When some backends succeed and others fail, the transformer can create a partial response:

```go
customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
    result := make(map[string]interface{})
    hasErrors := false
    
    // Process each response, handling errors individually
    for backend, resp := range responses {
        if resp.StatusCode < 400 {
            // Process successful response
            body, _ := io.ReadAll(resp.Body)
            var data map[string]interface{}
            json.Unmarshal(body, &data)
            result[backend] = data
        } else {
            // Note the error but continue processing other responses
            hasErrors = true
            result[backend] = map[string]interface{}{
                "error":      true,
                "statusCode": resp.StatusCode,
            }
        }
    }
    
    // Return partial content if some backends failed
    statusCode := http.StatusOK
    if hasErrors {
        statusCode = http.StatusPartialContent
        result["_meta"] = map[string]interface{}{
            "complete": false,
            "message": "Some backend services returned errors",
        }
    }
    
    resultJSON, _ := json.Marshal(result)
    return &reverseproxy.CompositeResponse{
        StatusCode: statusCode,
        Headers:    http.Header{"Content-Type": []string{"application/json"}},
        Body:       resultJSON,
    }, nil
}
```

### Timeout Management

The module handles timeouts for backend requests with configurable timeout durations:

1. **Global timeout configuration**:

```yaml
reverseproxy:
  backend_services:
    api: "http://api.example.com"
  timeout: 5  # 5-second timeout for all backend requests
```

2. **Per-route timeout configuration**:

```yaml
reverseproxy:
  composite_routes:
    "/api/slow-operation":
      pattern: "/api/slow-operation"
      backends: ["api", "data"]
      strategy: "merge"
      timeout: 30  # 30-second timeout for this specific route
```

3. **Handling timeout errors in transformers**:

```go
customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
    // Check if any backend timed out
    _, hasService1 := responses["service1"]
    _, hasService2 := responses["service2"]
    
    if !hasService1 || !hasService2 {
        // One or more services timed out
        return &reverseproxy.CompositeResponse{
            StatusCode: http.StatusGatewayTimeout,
            Headers:    http.Header{"Content-Type": []string{"application/json"}},
            Body:       []byte(`{"error":"Gateway timeout","message":"One or more backend services took too long to respond"}`),
        }, nil
    }
    
    // Process responses as normal
    // ...
}
```

### Redirect Handling

The module provides options for handling redirects from backend services:

1. **Default behavior**: Redirects are followed automatically up to a maximum number of redirects (typically 10).

2. **Custom redirect handling**: You can configure how redirects are handled in your response transformer:

```go
customTransformer := func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
    // Check for redirects
    for backend, resp := range responses {
        if resp.StatusCode >= 300 && resp.StatusCode < 400 {
            // Get the redirect location
            location := resp.Header.Get("Location")
            
            // Decide whether to follow the redirect or handle it differently
            if shouldFollowRedirect(location) {
                // Make a new request to the redirect location
                // ...
            } else {
                // Return the redirect directly to the client
                return &reverseproxy.CompositeResponse{
                    StatusCode: resp.StatusCode,
                    Headers:    resp.Header,
                    Body:       []byte{},
                }, nil
            }
        }
    }
    
    // Process non-redirect responses
    // ...
}
```

## API Reference

### Module Methods

#### `NewModule() (*ReverseProxyModule, error)`

Creates a new instance of the reverseproxy module.

#### `(m *ReverseProxyModule) RegisterCustomEndpoint(pattern string, mapping EndpointMapping)`

Registers a custom endpoint that maps to one or more backend endpoints with a response transformer.

Parameters:
- `pattern`: The URL pattern to match
- `mapping`: The endpoint mapping configuration

#### `(m *ReverseProxyModule) RegisterCompositeRoute(pattern string, route CompositeRoute)`

Registers a composite route based on configuration.

Parameters:
- `pattern`: The URL pattern to match
- `route`: The composite route configuration

### Handler Types

#### `BackendEndpointRequest`

Defines a request to be made to a backend service endpoint.

Properties:
- `Backend`: The identifier of the backend service
- `Path`: The path to request on the backend
- `Method`: The HTTP method to use (GET, POST, etc.)
- `QueryParams`: Map of query parameters to add to the request
- `Headers`: Map of headers to add or override in the request
- `Body`: Optional request body for POST/PUT requests

#### `EndpointMapping`

Maps a frontend endpoint to one or more backend endpoints.

Properties:
- `Endpoints`: Array of BackendEndpointRequest objects
- `ResponseTransformer`: Function that transforms the backend responses

#### `CompositeResponse`

Represents a response from a composite handler.

Properties:
- `StatusCode`: The HTTP status code
- `Headers`: The HTTP headers
- `Body`: The response body as bytes

### Response Combiners

#### `MergeJSONResponses(responses map[string]*http.Response) ([]byte, error)`

Merges multiple JSON responses into a single JSON object.

#### `SelectFirstSuccessfulResponse(responses map[string]*http.Response) (*http.Response, error)`

Returns the first successful response from the provided responses.

### Response Transformers

#### `ResponseTransformerFunc(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*CompositeResponse, error)`

Type definition for response transformer functions.

## Examples

### API Gateway Example

Using the module as a simple API gateway:

```go
func main() {
    // Create a new module
    proxyModule, _ := reverseproxy.NewModule()
    
    // Register it with the application
    app.RegisterModule(proxyModule)
    
    // Configure the module
    app.SetConfigPath("config.yaml")
    
    // Example config.yaml:
    /*
    reverseproxy:
      backend_services:
        users: "http://users-service:8080"
        products: "http://products-service:8080"
        orders: "http://orders-service:8080"
      default_backend: "users"
    */
    
    // Run the application
    app.Run()
}
```

### Response Aggregation Example

Aggregating data from multiple services:

```go
func main() {
    proxyModule, _ := reverseproxy.NewModule()
    app.RegisterModule(proxyModule)
    
    app.OnInit(func() {
        module := app.GetModule("reverseproxy").(*reverseproxy.ReverseProxyModule)
        
        // Register a custom endpoint
        module.RegisterCustomEndpoint("/api/product/{id}/details", reverseproxy.EndpointMapping{
            Endpoints: []reverseproxy.BackendEndpointRequest{
                {
                    Backend: "products",
                    Path:    "/api/products/{id}",
                    Method:  "GET",
                },
                {
                    Backend: "inventory",
                    Path:    "/api/stock/product/{id}",
                    Method:  "GET",
                },
                {
                    Backend: "reviews",
                    Path:    "/api/reviews/product/{id}",
                    Method:  "GET",
                    QueryParams: map[string]string{
                        "limit": "5",
                        "sort":  "recent",
                    },
                },
            },
            ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
                // Extract product details
                productBody, _ := io.ReadAll(responses["products"].Body)
                var product map[string]interface{}
                json.Unmarshal(productBody, &product)
                
                // Extract inventory data
                inventoryBody, _ := io.ReadAll(responses["inventory"].Body)
                var inventory map[string]interface{}
                json.Unmarshal(inventoryBody, &inventory)
                
                // Extract reviews
                reviewsBody, _ := io.ReadAll(responses["reviews"].Body)
                var reviews map[string]interface{}
                json.Unmarshal(reviewsBody, &reviews)
                
                // Create a combined response
                result := map[string]interface{}{
                    "product": product,
                    "inventory": inventory["stock"],
                    "reviews": reviews["items"],
                    "meta": map[string]interface{}{
                        "generated_at": time.Now().Format(time.RFC3339),
                    },
                }
                
                resultJSON, _ := json.Marshal(result)
                
                return &reverseproxy.CompositeResponse{
                    StatusCode: http.StatusOK,
                    Headers:    http.Header{"Content-Type": []string{"application/json"}},
                    Body:       resultJSON,
                }, nil
            },
        })
    })
    
    app.Run()
}
```

### Custom Transformation Example

Transforming and filtering backend responses:

```go
func main() {
    proxyModule, _ := reverseproxy.NewModule()
    app.RegisterModule(proxyModule)
    
    app.OnInit(func() {
        module := app.GetModule("reverseproxy").(*reverseproxy.ReverseProxyModule)
        
        module.RegisterCustomEndpoint("/api/user/{id}/public-profile", reverseproxy.EndpointMapping{
            Endpoints: []reverseproxy.BackendEndpointRequest{
                {
                    Backend: "users",
                    Path:    "/api/users/{id}/full-profile",
                    Method:  "GET",
                },
            },
            ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
                // Read the user profile
                profileBody, _ := io.ReadAll(responses["users"].Body)
                var fullProfile map[string]interface{}
                json.Unmarshal(profileBody, &fullProfile)
                
                // Create a filtered public profile
                publicProfile := map[string]interface{}{
                    "id":       fullProfile["id"],
                    "username": fullProfile["username"],
                    "name":     fullProfile["name"],
                    "bio":      fullProfile["bio"],
                    "avatar":   fullProfile["avatar_url"],
                    "joined":   fullProfile["created_at"],
                }
                
                // Add computed fields
                if joinedDate, err := time.Parse(time.RFC3339, publicProfile["joined"].(string)); err == nil {
                    publicProfile["member_for"] = fmt.Sprintf("%d years", int(time.Since(joinedDate).Hours()/24/365))
                }
                
                resultJSON, _ := json.Marshal(publicProfile)
                
                return &reverseproxy.CompositeResponse{
                    StatusCode: http.StatusOK,
                    Headers:    http.Header{"Content-Type": []string{"application/json"}},
                    Body:       resultJSON,
                }, nil
            },
        })
    })
    
    app.Run()
}
```

### Error Handling Example

Handling errors from backend services:

```go
module.RegisterCustomEndpoint("/api/critical-operation", reverseproxy.EndpointMapping{
    Endpoints: []reverseproxy.BackendEndpointRequest{
        {
            Backend: "service1",
            Path:    "/api/operation/part1",
            Method:  "POST",
        },
        {
            Backend: "service2",
            Path:    "/api/operation/part2",
            Method:  "POST",
        },
    },
    ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
        // Check for errors from both services
        if responses["service1"].StatusCode >= 400 || responses["service2"].StatusCode >= 400 {
            // Create detailed error response
            errorDetails := []map[string]interface{}{}
            
            if responses["service1"].StatusCode >= 400 {
                service1ErrorBody, _ := io.ReadAll(responses["service1"].Body)
                var service1Error map[string]interface{}
                json.Unmarshal(service1ErrorBody, &service1Error)
                
                errorDetails = append(errorDetails, map[string]interface{}{
                    "service":    "service1",
                    "statusCode": responses["service1"].StatusCode,
                    "details":    service1Error,
                })
            }
            
            if responses["service2"].StatusCode >= 400 {
                service2ErrorBody, _ := io.ReadAll(responses["service2"].Body)
                var service2Error map[string]interface{}
                json.Unmarshal(service2ErrorBody, &service2Error)
                
                errorDetails = append(errorDetails, map[string]interface{}{
                    "service":    "service2",
                    "statusCode": responses["service2"].StatusCode,
                    "details":    service2Error,
                })
            }
            
            errorResponse := map[string]interface{}{
                "success": false,
                "message": "Operation failed",
                "errors":  errorDetails,
            }
            
            errorJSON, _ := json.Marshal(errorResponse)
            
            return &reverseproxy.CompositeResponse{
                StatusCode: http.StatusBadGateway,
                Headers:    http.Header{"Content-Type": []string{"application/json"}},
                Body:       errorJSON,
            }, nil
        }
        
        // Process successful responses
        // ...
    },
})
```

### Partial Response Example

Handling cases where some backends succeed and others fail:

```go
module.RegisterCustomEndpoint("/api/dashboard", reverseproxy.EndpointMapping{
    Endpoints: []reverseproxy.BackendEndpointRequest{
        {
            Backend: "user-service",
            Path:    "/api/user/{id}",
            Method:  "GET",
        },
        {
            Backend: "stats-service",
            Path:    "/api/stats/{id}",
            Method:  "GET",
        },
        {
            Backend: "notifications-service",
            Path:    "/api/notifications/{id}/unread",
            Method:  "GET",
        },
    },
    ResponseTransformer: func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*reverseproxy.CompositeResponse, error) {
        dashboard := map[string]interface{}{}
        warnings := []string{}
        
        // Process user data
        if responses["user-service"].StatusCode == http.StatusOK {
            userBody, _ := io.ReadAll(responses["user-service"].Body)
            var userData map[string]interface{}
            json.Unmarshal(userBody, &userData)
            dashboard["user"] = userData
        } else {
            dashboard["user"] = nil
            warnings = append(warnings, "User data unavailable")
        }
        
        // Process stats
        if responses["stats-service"].StatusCode == http.StatusOK {
            statsBody, _ := io.ReadAll(responses["stats-service"].Body)
            var statsData map[string]interface{}
            json.Unmarshal(statsBody, &statsData)
            dashboard["stats"] = statsData
        } else {
            dashboard["stats"] = nil
            warnings = append(warnings, "Statistics unavailable")
        }
        
        // Process notifications
        if responses["notifications-service"].StatusCode == http.StatusOK {
            notifBody, _ := io.ReadAll(responses["notifications-service"].Body)
            var notifData map[string]interface{}
            json.Unmarshal(notifBody, &notifData)
            dashboard["notifications"] = notifData
        } else {
            dashboard["notifications"] = map[string]interface{}{
                "count": 0,
                "items": []interface{}{},
            }
            warnings = append(warnings, "Notifications unavailable")
        }
        
        // Add metadata
        dashboard["_meta"] = map[string]interface{}{
            "complete": len(warnings) == 0,
            "warnings": warnings,
            "generated_at": time.Now().Format(time.RFC3339),
        }
        
        dashboardJSON, _ := json.Marshal(dashboard)
        
        // Return HTTP 200 even with partial data, but include warnings
        return &reverseproxy.CompositeResponse{
            StatusCode: http.StatusOK,
            Headers:    http.Header{"Content-Type": []string{"application/json"}},
            Body:       dashboardJSON,
        }, nil
    },
})
```

## Performance Considerations

When using the Reverse Proxy module, consider the following performance factors:

1. **Backend Timeouts**: Set appropriate timeouts for backend services to prevent slow backends from affecting the overall response time.

2. **Parallel Requests**: Requests to multiple backends are made in parallel to minimize response time.

3. **Response Size**: Be cautious when combining large responses from multiple backends, as this can consume significant memory.

4. **Connection Reuse**: The module uses connection pooling to efficiently reuse connections to backend services.

5. **Resource Utilization**: Consider the CPU and memory impact when transforming large responses.

## Security Best Practices

1. **TLS Configuration**: When connecting to backend services over HTTPS, ensure proper TLS configuration:

   ```go
   customTransport := &http.Transport{
       TLSClientConfig: &tls.Config{
           MinVersion: tls.VersionTLS12,
       },
   }
   
   client := &http.Client{
       Transport: customTransport,
       Timeout:   10 * time.Second,
   }
   
   module.SetHttpClient(client)
   ```

2. **Header Filtering**: Be careful about which headers you forward to backend services to prevent information leakage.

3. **Request Validation**: Validate incoming requests before forwarding them to backend services.

4. **Error Information**: Avoid exposing sensitive information in error responses.

## Troubleshooting

### Common Issues

1. **Backend Connection Failures**:
   - Verify that the backend service URLs are correct and accessible
   - Check network connectivity and firewall rules
   - Verify that the backend services are running

2. **Timeout Errors**:
   - Increase the timeout configuration if backend services are slow
   - Check if backend services are overloaded
   - Consider implementing circuit breakers for unreliable backends

3. **Response Transformation Errors**:
   - Ensure that backend responses match the expected format
   - Add error handling in transformation functions
   - Log detailed error information for debugging

4. **Header Forwarding Issues**:
   - Verify that required headers are included in the `forward_headers` configuration
   - Check that backend services are receiving the expected headers

### Diagnostic Tools

1. **Enable Debug Logging**:

   ```go
   module.SetLogLevel(reverseproxy.LogLevelDebug)
   ```

2. **Request Tracing**: Implement request tracing with unique IDs across services.

3. **Response Inspection**: Log response details during development.

## FAQ

**Q: Can I use the Reverse Proxy module with HTTPS backends?**  
A: Yes, the module works with both HTTP and HTTPS backend services. Simply provide the HTTPS URL in the configuration.

**Q: How do I handle authentication in a microservices architecture?**  
A: You can forward authentication headers (like Authorization) to backend services, or implement a custom authentication scheme in your response transformers.

**Q: Can I implement rate limiting with this module?**  
A: While the module doesn't include built-in rate limiting, you can implement it by creating a custom middleware that wraps the proxy handlers.

**Q: How do I handle binary responses like file downloads?**  
A: For binary responses, you should use a transformer that properly handles binary data without attempting JSON parsing.

**Q: Can I use WebSockets through the proxy?**  
A: WebSocket support requires special configuration. Contact the module maintainers for specific guidance.