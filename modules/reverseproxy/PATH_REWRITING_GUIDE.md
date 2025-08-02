# ReverseProxy Module - Path Rewriting and Header Rewriting

## Overview

The reverseproxy module provides comprehensive path rewriting and header rewriting capabilities through per-backend and per-endpoint configuration. This approach gives you fine-grained control over how requests are transformed before being forwarded to backend services.

## Key Features

1. **Per-Backend Configuration**: Configure path rewriting and header rewriting for each backend service
2. **Per-Endpoint Configuration**: Override backend configuration for specific endpoints within a backend
3. **Hostname Handling**: Control how the Host header is handled (preserve original, use backend, or use custom)
4. **Header Rewriting**: Add, modify, or remove headers before forwarding requests
5. **Path Rewriting**: Transform request paths before forwarding to backends

## Configuration Structure

The path rewriting and header rewriting is configured through the `backend_configs` section:

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
    user: "http://user.internal.com"
  
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
        base_path_rewrite: "/internal/api"
      header_rewriting:
        hostname_handling: "preserve_original"
        set_headers:
          X-API-Key: "secret-key"
        remove_headers:
          - "X-Client-Version"
      
      endpoints:
        users:
          pattern: "/users/*"
          path_rewriting:
            base_path_rewrite: "/internal/users"
          header_rewriting:
            hostname_handling: "use_custom"
            custom_hostname: "users.internal.com"
```

## Path Rewriting Configuration

### Backend-Level Path Rewriting

Configure path rewriting for an entire backend service:

```yaml
backend_configs:
  api:
    path_rewriting:
      strip_base_path: "/api/v1"
      base_path_rewrite: "/internal/api"
```

#### Strip Base Path
Removes a specified base path from all requests to this backend:

- Request: `/api/v1/users/123` → Backend: `/users/123`
- Request: `/api/v1/orders/456` → Backend: `/orders/456`

#### Base Path Rewrite
Prepends a new base path to all requests to this backend:

- Request: `/users/123` → Backend: `/internal/api/users/123`
- Request: `/orders/456` → Backend: `/internal/api/orders/456`

#### Combined Strip and Rewrite
Both operations can be used together:

- Request: `/api/v1/users/123` → Backend: `/internal/api/users/123`

### Endpoint-Level Path Rewriting

Override backend-level configuration for specific endpoints:

```yaml
backend_configs:
  api:
    path_rewriting:
      strip_base_path: "/api/v1"
      base_path_rewrite: "/internal/api"
    
    endpoints:
      users:
        pattern: "/users/*"
        path_rewriting:
          base_path_rewrite: "/internal/users"  # Override backend setting
      
      orders:
        pattern: "/orders/*"
        path_rewriting:
          base_path_rewrite: "/internal/orders"
```

#### Pattern Matching

- **Exact Match**: `/api/users` matches only `/api/users`
- **Wildcard Match**: `/api/users/*` matches `/api/users/123`, `/api/users/123/profile`, etc.
- **Glob Patterns**: Supports glob pattern matching for flexible URL matching

#### Configuration Priority

Configuration is applied in order of precedence:
1. Endpoint-level configuration (highest priority)
2. Backend-level configuration
3. Default behavior (lowest priority)

## Header Rewriting Configuration

### Hostname Handling

Control how the Host header is handled when forwarding requests:

```yaml
backend_configs:
  api:
    header_rewriting:
      hostname_handling: "preserve_original"  # Default
      custom_hostname: "api.internal.com"     # Used with "use_custom"
```

#### Hostname Handling Options

- **`preserve_original`**: Preserves the original client's Host header (default)
- **`use_backend`**: Uses the backend service's hostname
- **`use_custom`**: Uses a custom hostname specified in `custom_hostname`

### Header Manipulation

Add, modify, or remove headers before forwarding requests:

```yaml
backend_configs:
  api:
    header_rewriting:
      set_headers:
        X-API-Key: "secret-key"
        X-Service: "api"
        X-Version: "v1"
      remove_headers:
        - "X-Client-Version"
        - "X-Debug-Mode"
```

#### Set Headers
- Adds new headers or overwrites existing ones
- Applies to all requests to this backend

#### Remove Headers
- Removes specified headers from requests
- Useful for removing sensitive client headers

### Endpoint-Level Header Rewriting

Override backend-level header configuration for specific endpoints:

```yaml
backend_configs:
  api:
    header_rewriting:
      hostname_handling: "preserve_original"
      set_headers:
        X-API-Key: "secret-key"
    
    endpoints:
      public:
        pattern: "/public/*"
        header_rewriting:
          set_headers:
            X-Auth-Required: "false"
          remove_headers:
            - "X-API-Key"  # Remove API key for public endpoints
```

## Tenant-Specific Configuration

Both path rewriting and header rewriting can be configured per tenant:

```yaml
# Global configuration
reverseproxy:
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
      header_rewriting:
        hostname_handling: "preserve_original"
        set_headers:
          X-API-Key: "global-key"

# Tenant-specific configuration
tenants:
  premium:
    reverseproxy:
      backend_configs:
        api:
          path_rewriting:
            strip_base_path: "/api/v2"  # Premium uses v2 API
            base_path_rewrite: "/premium/api"
          header_rewriting:
            set_headers:
              X-API-Key: "premium-key"
              X-Tenant-Type: "premium"
```

## Usage Examples

### Go Configuration
```go
config := &reverseproxy.ReverseProxyConfig{
    BackendServices: map[string]string{
        "api": "http://api.internal.com",
    },
    DefaultBackend: "api",
    
    BackendConfigs: map[string]reverseproxy.BackendServiceConfig{
        "api": {
            PathRewriting: reverseproxy.PathRewritingConfig{
                StripBasePath:   "/api/v1",
                BasePathRewrite: "/internal/api",
            },
            HeaderRewriting: reverseproxy.HeaderRewritingConfig{
                HostnameHandling: reverseproxy.HostnamePreserveOriginal,
                SetHeaders: map[string]string{
                    "X-API-Key": "secret-key",
                },
            },
            Endpoints: map[string]reverseproxy.EndpointConfig{
                "users": {
                    Pattern: "/users/*",
                    PathRewriting: reverseproxy.PathRewritingConfig{
                        BasePathRewrite: "/internal/users",
                    },
                    HeaderRewriting: reverseproxy.HeaderRewritingConfig{
                        HostnameHandling: reverseproxy.HostnameUseCustom,
                        CustomHostname:   "users.internal.com",
                    },
                },
            },
        },
    },
}
```

### Testing the Configuration

The module includes comprehensive test coverage for path rewriting and header rewriting. Key test scenarios include:

1. **Per-Backend Configuration Tests**: Verify backend-specific path and header rewriting
2. **Per-Endpoint Configuration Tests**: Test endpoint-specific overrides
3. **Hostname Handling Tests**: Verify different hostname handling modes
4. **Header Manipulation Tests**: Test setting and removing headers
5. **Tenant-Specific Tests**: Verify tenant-specific configurations work correctly
6. **Edge Cases**: Handle nil configurations, empty paths, pattern matching edge cases

## Key Benefits

1. **Fine-Grained Control**: Configure path and header rewriting per backend and endpoint
2. **Flexible Hostname Handling**: Choose how to handle the Host header for each backend
3. **Header Security**: Add, modify, or remove headers for security and functionality
4. **Multi-Tenant Support**: Tenant-specific configurations for complex routing scenarios
5. **Maintainable Configuration**: Clear separation between backend and endpoint concerns