# Per-Backend Configuration Guide

This guide explains how to configure path rewriting and header rewriting on a per-backend and per-endpoint basis in the reverseproxy module.

## Overview

The reverseproxy module now supports fine-grained configuration control:

1. **Per-Backend Configuration**: Configure path rewriting and header rewriting for specific backend services
2. **Per-Endpoint Configuration**: Configure path rewriting and header rewriting for specific endpoints within a backend
3. **Backward Compatibility**: Existing global configuration continues to work as before

## Configuration Structure

### Backend-Specific Configuration

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
    user: "http://user.internal.com"
  
  # Per-backend configuration
  backend_configs:
    api:
      url: "http://api.internal.com"  # Optional: can override backend_services URL
      path_rewriting:
        strip_base_path: "/api/v1"
        base_path_rewrite: "/internal/api"
      header_rewriting:
        hostname_handling: "preserve_original"  # Default
        set_headers:
          X-API-Key: "secret-key"
          X-Service: "api"
        remove_headers:
          - "X-Client-Version"
    
    user:
      url: "http://user.internal.com"
      path_rewriting:
        strip_base_path: "/user/v1"
        base_path_rewrite: "/internal/user"
      header_rewriting:
        hostname_handling: "use_backend"  # Use backend hostname
        set_headers:
          X-Service: "user"
```

### Endpoint-Specific Configuration

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
  
  backend_configs:
    api:
      # Backend-level configuration
      path_rewriting:
        strip_base_path: "/api/v1"
      header_rewriting:
        hostname_handling: "preserve_original"
        set_headers:
          X-API-Key: "secret-key"
      
      # Endpoint-specific configuration
      endpoints:
        users:
          pattern: "/users/*"
          path_rewriting:
            base_path_rewrite: "/internal/users"
          header_rewriting:
            hostname_handling: "use_custom"
            custom_hostname: "users.internal.com"
            set_headers:
              X-Endpoint: "users"
        
        orders:
          pattern: "/orders/*"
          path_rewriting:
            base_path_rewrite: "/internal/orders"
          header_rewriting:
            set_headers:
              X-Endpoint: "orders"
```

## Configuration Options

### Path Rewriting Options

- **`strip_base_path`**: Remove a base path from incoming requests
- **`base_path_rewrite`**: Add a new base path to requests
- **`endpoint_rewrites`**: Map of endpoint-specific rewriting rules (deprecated - use `endpoints` instead)

### Header Rewriting Options

- **`hostname_handling`**: How to handle the Host header
  - `preserve_original`: Keep the original client's Host header (default)
  - `use_backend`: Use the backend service's hostname
  - `use_custom`: Use a custom hostname specified in `custom_hostname`
- **`custom_hostname`**: Custom hostname to use when `hostname_handling` is `use_custom`
- **`set_headers`**: Map of headers to set or override
- **`remove_headers`**: List of headers to remove

## Configuration Priority

Configuration is applied in the following order (later overrides earlier):

1. **Global Configuration** (from `path_rewriting` in root config)
2. **Backend Configuration** (from `backend_configs[backend_id]`)
3. **Endpoint Configuration** (from `backend_configs[backend_id].endpoints[endpoint_id]`)

## Examples

### Example 1: API Gateway with Service-Specific Rewriting

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
    user: "http://user.internal.com"
    notification: "http://notification.internal.com"
  
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
        base_path_rewrite: "/internal/api"
      header_rewriting:
        hostname_handling: "preserve_original"
        set_headers:
          X-API-Version: "v1"
          X-Service: "api"
    
    user:
      path_rewriting:
        strip_base_path: "/user/v1"
        base_path_rewrite: "/internal/user"
      header_rewriting:
        hostname_handling: "use_backend"
        set_headers:
          X-Service: "user"
    
    notification:
      path_rewriting:
        strip_base_path: "/notification/v1"
        base_path_rewrite: "/internal/notification"
      header_rewriting:
        hostname_handling: "use_custom"
        custom_hostname: "notifications.internal.com"
        set_headers:
          X-Service: "notification"
```

**Request Transformations:**
- `/api/v1/products` → API backend: `/internal/api/products` with Host: `original.client.com`
- `/user/v1/profile` → User backend: `/internal/user/profile` with Host: `user.internal.com`
- `/notification/v1/send` → Notification backend: `/internal/notification/send` with Host: `notifications.internal.com`

### Example 2: Microservices with Endpoint-Specific Configuration

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
  
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
      header_rewriting:
        hostname_handling: "preserve_original"
        set_headers:
          X-API-Key: "global-api-key"
      
      endpoints:
        users:
          pattern: "/users/*"
          path_rewriting:
            base_path_rewrite: "/internal/users"
          header_rewriting:
            hostname_handling: "use_custom"
            custom_hostname: "users.internal.com"
            set_headers:
              X-Endpoint: "users"
              X-Auth-Required: "true"
        
        public:
          pattern: "/public/*"
          path_rewriting:
            base_path_rewrite: "/internal/public"
          header_rewriting:
            set_headers:
              X-Endpoint: "public"
              X-Auth-Required: "false"
            remove_headers:
              - "X-API-Key"  # Remove API key for public endpoints
```

**Request Transformations:**
- `/api/v1/users/123` → API backend: `/internal/users/123` with Host: `users.internal.com`
- `/api/v1/public/info` → API backend: `/internal/public/info` with Host: `original.client.com` (no API key header)
- `/api/v1/other/endpoint` → API backend: `/other/endpoint` with Host: `original.client.com` (uses backend-level config)

### Example 3: Tenant-Aware Configuration

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
  
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
      header_rewriting:
        hostname_handling: "preserve_original"

# Tenant-specific configuration
tenants:
  premium:
    reverseproxy:
      backend_configs:
        api:
          path_rewriting:
            strip_base_path: "/api/v2"  # Premium tenants use v2 API
            base_path_rewrite: "/premium"
          header_rewriting:
            set_headers:
              X-Tenant-Type: "premium"
              X-Rate-Limit: "10000"
  
  basic:
    reverseproxy:
      backend_configs:
        api:
          header_rewriting:
            set_headers:
              X-Tenant-Type: "basic"
              X-Rate-Limit: "1000"
```

## Migration from Global Configuration

### Before (Global Configuration)

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
  
  path_rewriting:
    strip_base_path: "/api/v1"
    base_path_rewrite: "/internal/api"
    endpoint_rewrites:
      users:
        pattern: "/users/*"
        replacement: "/internal/users"
        backend: "api"
```

### After (Per-Backend Configuration)

```yaml
reverseproxy:
  backend_services:
    api: "http://api.internal.com"
  
  backend_configs:
    api:
      path_rewriting:
        strip_base_path: "/api/v1"
        base_path_rewrite: "/internal/api"
      endpoints:
        users:
          pattern: "/users/*"
          path_rewriting:
            base_path_rewrite: "/internal/users"
```

## Best Practices

1. **Use Backend-Specific Configuration**: Configure path and header rewriting per backend for better organization
2. **Leverage Endpoint Configuration**: Use endpoint-specific configuration for fine-grained control
3. **Hostname Handling**: Choose appropriate hostname handling based on your backend requirements
4. **Header Security**: Use `remove_headers` to remove sensitive client headers before forwarding
5. **Tenant Configuration**: Use tenant-specific configuration for multi-tenant deployments

## Backward Compatibility

- All existing global `path_rewriting` configuration continues to work
- Global configuration is used as fallback when no backend-specific configuration is found
- New per-backend configuration takes precedence over global configuration
- No breaking changes to existing APIs