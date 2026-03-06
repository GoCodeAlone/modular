# Reverse Proxy Module

[![Go Reference](https://pkg.go.dev/badge/github.com/CrisisTextLine/modular/modules/reverseproxy.svg)](https://pkg.go.dev/github.com/CrisisTextLine/modular/modules/reverseproxy)

A module for the [Modular](https://github.com/CrisisTextLine/modular) framework that provides a flexible reverse proxy with advanced routing capabilities.

## Overview

The Reverse Proxy module functions as a versatile API gateway that can route requests to multiple backend services, combine responses, and support tenant-specific routing configurations. It's designed to be flexible, extensible, and easily configurable.

## Key Features

* **Multi-Backend Routing**: Route HTTP requests to any number of configurable backend services
* **Per-Backend Configuration**: Configure path rewriting and header rewriting for each backend service
* **Per-Endpoint Configuration**: Override backend configuration for specific endpoints within a backend
* **Feature Flag Support**: Control backend and route behavior using feature flags with optional alternatives
* **Hostname Handling**: Control how the Host header is handled (preserve original, use backend, or use custom)
* **Request Header Rewriting**: Add, modify, or remove headers before forwarding requests
* **Response Header Rewriting**: Add, modify, or remove response headers from backends (per-backend, per-endpoint, or globally)
* **Dynamic Response Header Modification**: Custom callback function to modify response headers based on backend, tenant, or response content
* **Path Rewriting**: Transform request paths before forwarding to backends
* **Response Aggregation**: Combine responses from multiple backends using various strategies
* **Custom Response Transformers**: Create custom functions to transform and merge backend responses
* **Tenant Awareness**: Support for multi-tenant environments with tenant-specific routing
* **Pattern-Based Routing**: Direct requests to specific backends based on URL patterns
* **Custom Endpoint Mapping**: Define flexible mappings from frontend endpoints to backend services
* **Pipeline Strategy**: Chain backend requests where each stage's response informs the next (map/reduce)
* **Fan-Out-Merge Strategy**: Parallel backend requests with custom ID-based response merging
* **Empty Response Policies**: Configurable handling of empty backend responses (allow, skip, or fail)
* **Health Checking**: Continuous monitoring of backend service availability with DNS resolution and HTTP checks
* **Circuit Breaker**: Automatic failure detection and recovery with configurable thresholds
* **Response Caching**: Performance optimization with TTL-based caching
* **Metrics Collection**: Comprehensive metrics for monitoring and debugging
* **Dry Run Mode**: Compare responses between different backends for testing and validation

## Installation

```go
go get github.com/CrisisTextLine/modular/modules/reverseproxy@v1.0.0
```

## Documentation

- **[Feature Flag Migration Guide](FEATURE_FLAG_MIGRATION_GUIDE.md)** - Migration guide for the new feature flag aggregator pattern
- **[Path Rewriting Guide](PATH_REWRITING_GUIDE.md)** - Detailed guide for configuring path transformations
- **[Per-Backend Configuration Guide](PER_BACKEND_CONFIGURATION_GUIDE.md)** - Advanced per-backend configuration options

## Usage

```go
package main

import (
	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
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

  # Basic routing
  routes:
    "/api/v1/*": "api"
    "/auth/*": "auth"
    "/user/*": "user"

  # Set the default backend
  default_backend: "api"

  # Tenant-specific configuration
  tenant_id_header: "X-Tenant-ID"
  require_tenant_id: false

  # Global timeout settings
  request_timeout: "30s"
  global_timeout: "60s"

  # Caching configuration
  cache_enabled: true
  cache_ttl: "5m"
```

### Advanced Configuration

Here's a comprehensive configuration example showcasing all available features:

```yaml
reverseproxy:
  # Backend service definitions
  backend_services:
    api-v1: "http://legacy-api.example.com"
    api-v2: "http://new-api.example.com"
    auth: "http://auth.example.com"
    user: "http://user-service.example.com"
    analytics: "http://analytics.example.com"

  # Basic routing
  routes:
    "/api/v1/*": "api-v1"
    "/api/v2/*": "api-v2"
    "/auth/*": "auth"
    "/user/*": "user"

  # Advanced route configurations with feature flags and alternatives
  route_configs:
    "/api/v2/*":
      feature_flag_id: "api-v2-enabled"        # Feature flag control
      alternative_backend: "api-v1"            # Fallback when disabled
      timeout: "45s"                           # Route-specific timeout
      path_rewrite: "/internal/api"            # Simple path rewriting
      dry_run: true                            # Enable comparison testing
      dry_run_backend: "api-v1"               # Backend to compare against

  # Per-backend configuration
  backend_configs:
    api-v2:
      # Advanced path rewriting
      path_rewriting:
        strip_base_path: "/api/v2"
        base_path_rewrite: "/internal/api/v2"
        endpoint_rewrites:
          users_endpoint:
            pattern: "/users"
            replacement: "/internal/users"
            strip_query_params: true

      # Request header management
      header_rewriting:
        hostname_handling: "use_custom"
        custom_hostname: "api-internal.example.com"
        set_headers:
          X-API-Version: "2.0"
          X-Service-Name: "api-v2"
        remove_headers: ["X-Legacy-Header"]

      # Response header management (new feature)
      response_header_rewriting:
        set_headers:
          Access-Control-Allow-Origin: "*"
          Access-Control-Allow-Methods: "GET, POST, PUT, DELETE, OPTIONS"
          Access-Control-Allow-Headers: "Content-Type, Authorization"
          X-Backend-Version: "2.0"
        remove_headers: ["X-Internal-Header", "X-Debug-Info"]

      # Health check configuration
      health_check:
        enabled: true
        interval: "30s"
        timeout: "5s"
        expected_status_codes: [200, 204]
      health_endpoint: "/health"

      # Circuit breaker configuration
      circuit_breaker:
        enabled: true
        failure_threshold: 5
        recovery_timeout: "60s"

      # Retry configuration
      max_retries: 3
      retry_delay: "1s"

      # Connection pool settings
      max_connections: 100
      connection_timeout: "10s"
      idle_timeout: "30s"

      # Queue configuration
      queue_size: 1000
      queue_timeout: "5s"

      # Feature flag support
      feature_flag_id: "backend-v2-enabled"
      alternative_backend: "api-v1"

      # Endpoint-specific overrides
      endpoints:
        users:
          pattern: "/users/*"
          path_rewriting:
            base_path_rewrite: "/internal/users/v2"
          header_rewriting:
            set_headers:
              X-Endpoint: "users"
          feature_flag_id: "users-v2-enabled"
          alternative_backend: "api-v1"

    auth:
      header_rewriting:
        hostname_handling: "use_backend"
        set_headers:
          X-Service: "auth"
      health_endpoint: "/status"
      circuit_breaker:
        enabled: true
        failure_threshold: 3
        recovery_timeout: "30s"

  # Composite routes for response aggregation
  composite_routes:
    "/api/user/profile":
      pattern: "/api/user/profile"
      backends: ["user", "analytics"]
      strategy: "merge"
      feature_flag_id: "enhanced-profile"      # Feature flag for composite routes
      alternative_backend: "user"              # Single backend fallback

  # Global health check configuration
  health_check:
    enabled: true
    interval: "30s"
    timeout: "5s"
    recent_request_threshold: "60s"
    expected_status_codes: [200, 204]
    health_endpoints:
      api-v1: "/health"
      api-v2: "/v2/health"
      auth: "/status"
    backend_health_check_config:
      api-v2:
        enabled: true
        interval: "15s"
        timeout: "3s"
        expected_status_codes: [200]
      auth:
        endpoint: "/status"
        interval: "45s"
        timeout: "10s"
        expected_status_codes: [200, 201]

  # Circuit breaker configuration (global defaults)
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    success_threshold: 2
    open_timeout: "60s"
    half_open_allowed_requests: 3
    window_size: 10
    success_rate_threshold: 0.6

  # Per-backend circuit breaker overrides
  backend_circuit_breakers:
    api-v2:
      enabled: true
      failure_threshold: 3
      open_timeout: "30s"

  # Feature flags configuration
  feature_flags:
    enabled: true
    flags:
      api-v2-enabled: false
      backend-v2-enabled: true
      enhanced-profile: true
      users-v2-enabled: false

  # Metrics configuration
  metrics_enabled: true
  metrics_path: "/metrics"
  metrics_endpoint: "/metrics"
  metrics_config:
    enabled: true
    endpoint: "/metrics"

  # Debug endpoints configuration
  debug_endpoints:
    enabled: true
    base_path: "/debug"
    require_auth: true
    auth_token: "debug-secret-token"

  debug_config:
    enabled: true
    info_endpoint: "/debug/info"
    backends_endpoint: "/debug/backends"
    flags_endpoint: "/debug/flags"
    circuit_breakers_endpoint: "/debug/circuit-breakers"
    health_checks_endpoint: "/debug/health-checks"

  # Dry run configuration
  dry_run:
    enabled: true
    log_responses: true
    max_response_size: 1048576              # 1MB
    compare_headers: ["Content-Type", "X-Custom-Header"]
    ignore_headers: ["Date", "X-Request-ID", "Server"]
    default_response_backend: "primary"

  # Global header management
  header_config:
    set_headers:
      X-Proxy: "modular-reverse-proxy"
      X-Environment: "production"
    remove_headers: ["X-Internal-Only"]

  # Error handling configuration
  error_handling:
    enable_custom_pages: true
    retry_attempts: 2
    connection_retries: 1
    retry_delay: "500ms"

  # Tenant configuration
  tenant_id_header: "X-Tenant-ID"
  require_tenant_id: false

  # Timeout configuration
  request_timeout: "30s"
  global_timeout: "60s"

  # Cache configuration
  cache_enabled: true
  cache_ttl: "5m"
```

### Advanced Features

The module supports several advanced features:

1. **Response Header Rewriting**: Modify, add, or remove response headers from backends (globally, per-backend, or per-endpoint)
2. **Dynamic Response Modification**: Custom callback functions to modify response headers based on backend, tenant, or response content
3. **Custom Response Transformers**: Create custom functions to transform responses from multiple backends
4. **Custom Endpoint Mappings**: Define detailed mappings between frontend endpoints and backend services
5. **Tenant-Specific Routing**: Route requests to different backend URLs based on tenant ID
6. **Health Checking**: Continuous monitoring of backend service availability with configurable endpoints and intervals
7. **Circuit Breaker**: Automatic failure detection and recovery to prevent cascading failures
8. **Response Caching**: Performance optimization with TTL-based caching of responses
9. **Feature Flags**: Control backend and route behavior dynamically using feature flag evaluation
10. **Debug Endpoints**: Comprehensive debugging and monitoring endpoints
11. **Connection Pooling**: Advanced connection pool management with configurable limits
12. **Queue Management**: Request queueing with configurable sizes and timeouts
13. **Error Handling**: Comprehensive error handling with custom pages and retry logic
14. **Pipeline Strategy**: Chain backend requests where each stage's response informs the next request (map/reduce pattern)
15. **Fan-Out-Merge Strategy**: Parallel backend requests with custom ID-based response merging
16. **Empty Response Policies**: Configurable handling of empty backend responses (allow, skip, or fail)

### Composite Route Strategies

Composite routes allow combining responses from multiple backend services. The module supports five strategies:

#### first-success
Tries backends sequentially until one succeeds. Use case: High-availability setup with primary and fallback backends.

```yaml
composite_routes:
  "/api/data":
    pattern: "/api/data"
    backends: ["primary-backend", "fallback-backend"]
    strategy: "first-success"
```

#### merge
Executes all backend requests in parallel and merges JSON responses by backend ID.

```yaml
composite_routes:
  "/api/user/profile":
    pattern: "/api/user/profile"
    backends: ["user-backend", "analytics-backend"]
    strategy: "merge"
```

#### sequential
Executes requests one at a time, returning the last successful response.

```yaml
composite_routes:
  "/api/process":
    pattern: "/api/process"
    backends: ["auth-backend", "processing-backend"]
    strategy: "sequential"
```

#### pipeline
Executes backends sequentially where each stage's response can inform the next stage's request. Requires programmatic configuration via `SetPipelineConfig()`.

Use case: A list page shows queued conversations. Backend A returns conversation details, those IDs are fed into Backend B to fetch follow-up information, and the responses are merged.

```yaml
composite_routes:
  "/api/conversations":
    pattern: "/api/conversations"
    backends: ["conversations-backend", "followup-backend"]
    strategy: "pipeline"
    empty_policy: "skip-empty"  # Optional: allow-empty, skip-empty, fail-on-empty
```

```go
proxyModule.SetPipelineConfig("/api/conversations", reverseproxy.PipelineConfig{
    RequestBuilder: func(ctx context.Context, originalReq *http.Request, 
        previousResponses map[string][]byte, nextBackendID string) (*http.Request, error) {
        // Extract IDs from previous response and build next request
        var convResp struct {
            Conversations []struct{ ID string `json:"id"` } `json:"conversations"`
        }
        json.Unmarshal(previousResponses["conversations-backend"], &convResp)
        
        ids := []string{}
        for _, c := range convResp.Conversations {
            ids = append(ids, c.ID)
        }
        url := "http://followup-service/followups?ids=" + strings.Join(ids, ",")
        return http.NewRequestWithContext(ctx, "GET", url, nil)
    },
    ResponseMerger: func(ctx context.Context, originalReq *http.Request,
        allResponses map[string][]byte) (*http.Response, error) {
        // Merge follow-up data into conversations
        // ... custom merging logic ...
        return reverseproxy.MakeJSONResponse(http.StatusOK, mergedResult)
    },
})
```

#### fan-out-merge
Executes all backends in parallel (like merge), then applies a custom merger function for ID-based matching, filtering, or complex data correlation. Requires programmatic configuration via `SetFanOutMerger()`.

Use case: A ticket dashboard where tickets come from one service and priority/assignment data comes from another. The merger matches by ticket ID.

```yaml
composite_routes:
  "/api/tickets":
    pattern: "/api/tickets"
    backends: ["tickets-backend", "assignments-backend"]
    strategy: "fan-out-merge"
    empty_policy: "allow-empty"  # Optional
```

```go
proxyModule.SetFanOutMerger("/api/tickets", func(ctx context.Context,
    originalReq *http.Request, responses map[string][]byte) (*http.Response, error) {
    // Parse both responses
    var ticketsResp struct { Tickets []map[string]interface{} `json:"tickets"` }
    json.Unmarshal(responses["tickets-backend"], &ticketsResp)
    
    var assignResp struct { Assignments map[string]interface{} `json:"assignments"` }
    json.Unmarshal(responses["assignments-backend"], &assignResp)
    
    // Merge by ID
    for i, ticket := range ticketsResp.Tickets {
        if id, ok := ticket["id"].(string); ok {
            if assignment, exists := assignResp.Assignments[id]; exists {
                ticketsResp.Tickets[i]["assignment"] = assignment
            }
        }
    }
    return reverseproxy.MakeJSONResponse(http.StatusOK, map[string]interface{}{
        "tickets": ticketsResp.Tickets,
    })
})
```

#### Empty Response Policies

For `pipeline` and `fan-out-merge` strategies, you can control how empty backend responses are handled:

| Policy | Description |
|--------|-------------|
| `allow-empty` | Include empty responses in the result set (default) |
| `skip-empty` | Silently drop empty responses from the result |
| `fail-on-empty` | Fail the entire request if any backend returns empty |

Set via config (`empty_policy` field) or programmatically:
```go
proxyModule.SetEmptyResponsePolicy("/api/route", reverseproxy.EmptyResponseSkip)
```

### Debug Endpoints

The reverse proxy module provides comprehensive debug endpoints for monitoring and troubleshooting:

```yaml
reverseproxy:
  debug_endpoints:
    enabled: true                        # Enable debug endpoints
    base_path: "/debug"                  # Base path for debug endpoints
    require_auth: true                   # Require authentication
    auth_token: "your-debug-token"       # Auth token (if require_auth is true)

  debug_config:
    enabled: true                        # Enable individual debug endpoints
    info_endpoint: "/debug/info"         # General proxy information
    backends_endpoint: "/debug/backends" # Backend status information
    flags_endpoint: "/debug/flags"       # Feature flag status
    circuit_breakers_endpoint: "/debug/circuit-breakers"  # Circuit breaker status
    health_checks_endpoint: "/debug/health-checks"        # Health check status
```

**Available Debug Endpoints:**
- `GET /debug/info` - General reverse proxy information and configuration
- `GET /debug/backends` - Backend service status and configuration
- `GET /debug/flags` - Current feature flag values for the tenant
- `GET /debug/circuit-breakers` - Circuit breaker states and failure counts
- `GET /debug/health-checks` - Health check status and timing information

**Authentication:**
When `require_auth` is enabled, include the auth token in the request:
```bash
curl -H "Authorization: Bearer your-debug-token" http://localhost:8080/debug/info
```

### Response Header Rewriting

The reverse proxy module supports comprehensive response header rewriting at multiple levels: global, per-backend, and per-endpoint. This is particularly useful for consolidating CORS headers, adding security headers, or removing internal headers from backend responses.

#### Configuration Levels

Response headers are applied in the following order (later levels override earlier ones):
1. **Global Configuration**: Applied to all responses
2. **Backend Configuration**: Applied to responses from a specific backend
3. **Endpoint Configuration**: Applied to responses from a specific endpoint within a backend

#### Basic Usage

**Global Response Headers** (applied to all backends):
```yaml
reverseproxy:
  response_header_config:
    set_headers:
      X-Proxy-Version: "1.0"
      X-Powered-By: "Modular-ReverseProxy"
    remove_headers:
      - "X-Internal-Debug"
```

**Per-Backend Response Headers**:
```yaml
reverseproxy:
  backend_configs:
    legacy-api:
      url: "http://legacy.example.com"
      response_header_rewriting:
        set_headers:
          Access-Control-Allow-Origin: "*"
          Access-Control-Allow-Methods: "GET, POST, PUT, DELETE, OPTIONS"
          Access-Control-Allow-Headers: "Content-Type, Authorization"
          Access-Control-Max-Age: "86400"
        remove_headers:
          - "X-Internal-Header"
          - "X-Debug-Info"
```

**Per-Endpoint Response Headers** (highest priority):
```yaml
reverseproxy:
  backend_configs:
    api:
      url: "http://api.example.com"
      response_header_rewriting:
        set_headers:
          Cache-Control: "public, max-age=300"
      endpoints:
        /api/sensitive:
          pattern: "/api/sensitive"
          response_header_rewriting:
            set_headers:
              Cache-Control: "no-cache, no-store, must-revalidate"
              X-Content-Type-Options: "nosniff"
```

#### CORS Header Consolidation Use Case

A common use case is to consolidate or override CORS headers from multiple backends:

```yaml
reverseproxy:
  backend_configs:
    # Legacy backend with inconsistent CORS headers
    legacy-api:
      url: "http://legacy.example.com"
      response_header_rewriting:
        set_headers:
          # Override backend CORS headers with consistent values
          Access-Control-Allow-Origin: "*"
          Access-Control-Allow-Methods: "GET, POST, PUT, DELETE, OPTIONS"
          Access-Control-Allow-Headers: "Content-Type, Authorization, X-Tenant-ID"
          Access-Control-Max-Age: "86400"
          Access-Control-Allow-Credentials: "true"
```

#### Dynamic Response Header Modification

For more complex scenarios, you can use a custom callback function to modify response headers dynamically based on backend, tenant, or response content:

```go
proxyModule := reverseproxy.NewModule()

// Set a custom response header modifier
proxyModule.SetResponseHeaderModifier(func(resp *http.Response, backendID string, tenantID modular.TenantID) error {
    // Add backend information
    resp.Header.Set("X-Backend-ID", backendID)
    
    // Add tenant information if available
    if tenantID != "" {
        resp.Header.Set("X-Tenant-ID", string(tenantID))
    }
    
    // Dynamically set caching based on status code
    if resp.StatusCode == http.StatusOK {
        resp.Header.Set("Cache-Control", "public, max-age=300")
    } else {
        resp.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
    }
    
    // Consolidate duplicate CORS headers
    if len(resp.Header.Values("Access-Control-Allow-Origin")) > 1 {
        resp.Header.Del("Access-Control-Allow-Origin")
        resp.Header.Set("Access-Control-Allow-Origin", "*")
    }
    
    return nil
})
```

#### Tenant-Specific Response Headers

Response headers can also be configured per-tenant by registering tenant-specific configurations:

```go
tenantService.RegisterTenant("tenant1", map[string]modular.ConfigProvider{
    "reverseproxy": modular.NewStdConfigProvider(&reverseproxy.ReverseProxyConfig{
        BackendConfigs: map[string]BackendServiceConfig{
            "api": {
                URL: "http://api.example.com",
                ResponseHeaderRewriting: ResponseHeaderRewritingConfig{
                    SetHeaders: map[string]string{
                        "X-Tenant": "tenant1",
                        "Access-Control-Allow-Origin": "https://tenant1.example.com",
                    },
                },
            },
        },
    }),
})
```

#### Use Cases

Response header rewriting is particularly useful for:

1. **CORS Header Consolidation**: Override inconsistent or incorrect CORS headers from backends
2. **Security Headers**: Add headers like `X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`
3. **Cache Control**: Add or modify cache control headers based on backend or response status
4. **Header Sanitization**: Remove internal or debug headers before responses reach clients
5. **Multi-Tenant Headers**: Add tenant-specific headers to responses
6. **API Versioning**: Add version information headers to responses
7. **Compliance**: Ensure all responses meet security and compliance requirements

### Connection Pool Management

Advanced connection pool configuration for backend services:

```yaml
reverseproxy:
  backend_configs:
    api:
      # Connection pool settings
      max_connections: 100              # Maximum concurrent connections
      connection_timeout: "10s"         # Connection establishment timeout
      idle_timeout: "30s"              # Idle connection timeout

      # Queue configuration for connection limits
      queue_size: 1000                  # Maximum queued requests
      queue_timeout: "5s"               # Maximum time to wait in queue
```

**Connection Pool Features:**
- **Maximum Connections**: Limit concurrent connections per backend
- **Connection Timeouts**: Configure connection establishment timeouts
- **Idle Timeouts**: Automatically close idle connections
- **Request Queueing**: Queue requests when connection limits are reached
- **Queue Timeouts**: Prevent requests from waiting indefinitely

### Error Handling Configuration

Comprehensive error handling with custom pages and retry logic:

```yaml
reverseproxy:
  error_handling:
    enable_custom_pages: true           # Enable custom error pages
    retry_attempts: 2                   # Number of retry attempts
    connection_retries: 1               # Connection-specific retries
    retry_delay: "500ms"               # Delay between retry attempts
```

**Error Handling Features:**
- **Custom Error Pages**: Serve custom error pages for different HTTP status codes
- **Intelligent Retries**: Retry failed requests with configurable delays
- **Connection Retries**: Separate retry logic for connection failures
- **Backoff Strategies**: Configurable delay between retry attempts

### Circuit Breaker Enhancements

Enhanced circuit breaker configuration with per-backend overrides:

```yaml
reverseproxy:
  # Global circuit breaker defaults
  circuit_breaker:
    enabled: true
    failure_threshold: 5                # Number of failures before opening
    success_threshold: 2                # Successes needed to close circuit
    open_timeout: "60s"                # Time to wait before half-open
    half_open_allowed_requests: 3       # Requests allowed in half-open state
    window_size: 10                     # Size of the sliding window
    success_rate_threshold: 0.6         # Success rate required (0.0-1.0)

  # Per-backend circuit breaker overrides
  backend_circuit_breakers:
    critical-service:
      enabled: true
      failure_threshold: 3              # More sensitive for critical services
      open_timeout: "30s"              # Faster recovery attempts

    legacy-service:
      enabled: true
      failure_threshold: 10             # More tolerant for legacy services
      success_rate_threshold: 0.4       # Lower success rate requirement

  # Backend-specific circuit breaker config
  backend_configs:
    api:
      circuit_breaker:
        enabled: true
        failure_threshold: 5
        recovery_timeout: "60s"
```

**Circuit Breaker Features:**
- **Global Configuration**: Set default circuit breaker parameters for all backends
- **Per-Backend Overrides**: Customize circuit breaker settings for specific backends
- **Sliding Window**: Track failures over a configurable time window
- **Success Rate Monitoring**: Monitor success rates in addition to failure counts
- **Half-Open Testing**: Gradually test recovery with limited requests
- **Automatic Recovery**: Automatically attempt to close circuits based on success metrics

### Metrics and Monitoring

Comprehensive metrics collection and monitoring capabilities:

```yaml
reverseproxy:
  # Basic metrics configuration
  metrics_enabled: true
  metrics_path: "/metrics"              # Deprecated, use metrics_config instead
  metrics_endpoint: "/metrics"          # Deprecated, use metrics_config instead

  # Enhanced metrics configuration
  metrics_config:
    enabled: true
    endpoint: "/metrics"                # Prometheus-compatible metrics endpoint

  # Debug endpoints for real-time monitoring
  debug_config:
    enabled: true
    info_endpoint: "/debug/info"
    backends_endpoint: "/debug/backends"
    circuit_breakers_endpoint: "/debug/circuit-breakers"
    health_checks_endpoint: "/debug/health-checks"
```

**Available Metrics:**
- **Request Metrics**: Request count, response times, status codes
- **Backend Metrics**: Backend availability, response times, error rates
- **Circuit Breaker Metrics**: Circuit states, failure counts, recovery times
- **Health Check Metrics**: Health check success rates, response times
- **Connection Pool Metrics**: Active connections, queue sizes, timeouts
- **Cache Metrics**: Cache hit rates, eviction counts, TTL statistics

**Metric Endpoints:**
- `GET /metrics` - Prometheus-compatible metrics (if metrics enabled)
- `GET /debug/info` - JSON-formatted proxy statistics
- `GET /debug/backends` - Backend-specific metrics and status
- `GET /debug/circuit-breakers` - Real-time circuit breaker status
- `GET /debug/health-checks` - Health check timing and status information

### Feature Flag Support

The reverse proxy module supports feature flags to control routing behavior dynamically. Feature flags can be used to:

- Enable/disable specific backends
- Route to alternative backends when features are disabled  
- Control composite route availability
- Support A/B testing and gradual rollouts
- Provide tenant-specific feature access

#### Feature Flag Configuration

```yaml
reverseproxy:
  # Backend configurations with feature flags
  backend_configs:
    api-v2:
      feature_flag_id: "api-v2-enabled"     # Feature flag to check
      alternative_backend: "api-v1"         # Fallback when disabled
    
    beta-features:
      feature_flag_id: "beta-features"
      alternative_backend: "stable-api"
  
  # Composite routes with feature flags
  composite_routes:
    "/api/enhanced":
      backends: ["api-v2", "analytics"]
      strategy: "merge"
      feature_flag_id: "enhanced-api"       # Feature flag for composite route
      alternative_backend: "api-v1"         # Single backend fallback
```

#### Feature Flag Evaluator Service

The reverse proxy module uses an **aggregator pattern** for feature flag evaluation, allowing multiple evaluators to work together with priority-based ordering:

**Built-in File Evaluator**: Automatically available using tenant-aware configuration (lowest priority, fallback).

**External Evaluators**: Register additional evaluators by implementing the `FeatureFlagEvaluator` interface. The service name doesn't matter for discovery - the aggregator finds evaluators by interface matching:

```go
// Register a remote feature flag service
type RemoteEvaluator struct{}
func (r *RemoteEvaluator) Weight() int { return 50 } // Higher priority than file evaluator
func (r *RemoteEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
    // Custom logic here
    return true, nil
}
func (r *RemoteEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
    enabled, err := r.EvaluateFlag(ctx, flagID, tenantID, req)
    if err != nil { return defaultValue }
    return enabled
}

// Register with any service name (name doesn't matter for discovery)
app.RegisterService("remoteEvaluator", &RemoteEvaluator{})
// or  
app.RegisterService("my-custom-flags", &RemoteEvaluator{})
```

The aggregator automatically discovers all services implementing `FeatureFlagEvaluator` interface regardless of their registered name. If multiple evaluators have the same name, unique names are automatically generated. Evaluators are called in priority order (lower weight = higher priority), with the built-in file evaluator (weight: 1000) serving as the final fallback.

**Migration Note**: External evaluators are now discovered by interface matching rather than naming patterns. You can use any service name when registering. See the [Feature Flag Migration Guide](FEATURE_FLAG_MIGRATION_GUIDE.md) for detailed migration instructions.

The evaluator interface supports integration with external feature flag services like LaunchDarkly, Split.io, or custom implementations.

### Dry Run Mode

Dry run mode enables you to compare responses between different backends, which is particularly useful for testing new services, validating migrations, or A/B testing. When dry run is enabled for a route, requests are sent to both the primary and comparison backends, but only one response is returned to the client while differences are logged for analysis.

#### Basic Dry Run Configuration

```yaml
reverseproxy:
  backend_services:
    legacy: "http://legacy.service.com"
    v2: "http://new.service.com"
  
  routes:
    "/api/users": "v2"  # Primary route goes to v2
  
  route_configs:
    "/api/users":
      feature_flag_id: "v2-users-api"
      alternative_backend: "legacy"
      dry_run: true
      dry_run_backend: "v2"  # Backend to compare against
  
  dry_run:
    enabled: true
    log_responses: true
    max_response_size: 1048576  # 1MB
```

#### Dry Run with Feature Flags

The most powerful use case combines dry run with feature flags:

```yaml
feature_flags:
  enabled: true
  flags:
    v2-users-api: false  # Feature flag disabled

route_configs:
  "/api/users":
    feature_flag_id: "v2-users-api"
    alternative_backend: "legacy"
    dry_run: true
    dry_run_backend: "v2"
```

**Behavior when feature flag is disabled:**
- Returns response from `alternative_backend` (legacy)
- Compares with `dry_run_backend` (v2) in background
- Logs differences for analysis

**Behavior when feature flag is enabled:**
- Returns response from primary backend (v2)  
- Compares with `dry_run_backend` or `alternative_backend`
- Logs differences for analysis

#### Dry Run Configuration Options

```yaml
dry_run:
  enabled: true                          # Enable dry run globally
  log_responses: true                    # Log response bodies (can be verbose)
  max_response_size: 1048576            # Maximum response size to compare
  compare_headers: ["Content-Type"]      # Specific headers to compare
  ignore_headers: ["Date", "X-Request-ID"]  # Headers to ignore in comparison
  default_response_backend: "primary"   # Which response to return ("primary" or "secondary")
```

#### Use Cases

1. **Service Migration**: Test new service implementations while serving traffic from stable backend
2. **A/B Testing**: Compare different service versions with real traffic
3. **Validation**: Ensure new services produce equivalent responses to legacy systems
4. **Performance Testing**: Compare response times between different backends
5. **Gradual Rollout**: Safely test new features while maintaining fallback options

#### Monitoring Dry Run Results

Dry run comparisons are logged with detailed information:

```json
{
  "operation": "dry-run",
  "endpoint": "/api/users", 
  "primaryBackend": "legacy",
  "secondaryBackend": "v2",
  "statusCodeMatch": true,
  "headersMatch": false,
  "bodyMatch": false,
  "differences": ["Response body content differs"],
  "primaryResponseTime": "45ms",
  "secondaryResponseTime": "32ms"
}
```

Use these logs to identify discrepancies and validate that your new services work correctly before fully switching over.

### Health Check Configuration

The reverseproxy module provides comprehensive health checking capabilities:

```yaml
health_check:
  enabled: true                    # Enable health checking
  interval: "30s"                  # Global check interval
  timeout: "5s"                    # Global check timeout
  recent_request_threshold: "60s"  # Skip checks if recent request within threshold
  expected_status_codes: [200, 204] # Global expected status codes
  
  # Custom health endpoints per backend
  health_endpoints:
    api: "/health"
    auth: "/api/health"
  
  # Per-backend health check configuration
  backend_health_check_config:
    api:
      enabled: true
      interval: "15s"              # Override global interval
      timeout: "3s"                # Override global timeout
      expected_status_codes: [200] # Override global status codes
    auth:
      enabled: true
      endpoint: "/status"          # Custom health endpoint
      interval: "45s"
      timeout: "10s"
      expected_status_codes: [200, 201]
```

**Health Check Features:**
- **DNS Resolution**: Verifies that backend hostnames resolve to IP addresses
- **HTTP Connectivity**: Tests HTTP connectivity to backends with configurable timeouts
- **Custom Endpoints**: Supports custom health check endpoints per backend
- **Smart Scheduling**: Skips health checks if recent requests have occurred
- **Per-Backend Configuration**: Allows fine-grained control over health check behavior
- **Status Monitoring**: Tracks health status, response times, and error details
- **Metrics Integration**: Exposes health status through metrics endpoints

1. **Per-Backend Configuration**: Configure path rewriting and header rewriting for each backend service
2. **Per-Endpoint Configuration**: Override backend configuration for specific endpoints
3. **Hostname Handling**: Control how the Host header is handled for each backend
4. **Header Rewriting**: Add, modify, or remove headers before forwarding requests
5. **Path Rewriting**: Transform request paths before forwarding to backends
6. **Custom Response Transformers**: Create custom functions to transform responses from multiple backends
7. **Custom Endpoint Mappings**: Define detailed mappings between frontend endpoints and backend services
8. **Tenant-Specific Routing**: Route requests to different backend URLs based on tenant ID

For detailed documentation and examples, see:
- [PATH_REWRITING_GUIDE.md](PATH_REWRITING_GUIDE.md) - Complete guide to path rewriting and header rewriting
- [PER_BACKEND_CONFIGURATION_GUIDE.md](PER_BACKEND_CONFIGURATION_GUIDE.md) - Per-backend and per-endpoint configuration
- [DOCUMENTATION.md](DOCUMENTATION.md) - General module documentation

## License

[MIT License](LICENSE)
