# Reverse Proxy Example

This example demonstrates how to create a reverse proxy server using the modular framework with tenant-specific default backend routing.

## What it demonstrates

- **Tenant-Specific Default Backends**: Different tenants can have different default backend services
- **Reverse Proxy Configuration**: Setting up backend services and routing rules
- **ChiMux Integration**: Using the Chi router with CORS middleware
- **HTTP Server Module**: Configuring and running an HTTP server
- **Module Composition**: How multiple modules work together seamlessly
- **CORS Handling**: Cross-origin resource sharing configuration
- **Graceful Shutdown**: Proper application lifecycle management

## Features

- **Tenant-Aware Routing**: Route requests to different backends based on tenant ID
- HTTP reverse proxy with configurable backend services
- Chi router with CORS middleware
- Configurable CORS policies (origins, methods, headers, credentials)
- Multiple backend service support
- Mock backend servers for testing
- Graceful shutdown handling

## Running the Example

```bash
cd examples/reverse-proxy

# Build the application
go build -o reverse-proxy .

# Run the reverse proxy server
./reverse-proxy
```

The server will start on `localhost:8080` by default, along with 4 mock backend servers:
- **Global Default Backend** (port 9001): `{"backend":"global-default"}`
- **Tenant1 Backend** (port 9002): `{"backend":"tenant1-backend"}`
- **Tenant2 Backend** (port 9003): `{"backend":"tenant2-backend"}`
- **Specific API Backend** (port 9004): `{"backend":"specific-api"}`

## Testing Tenant-Specific Routing

You can test the tenant-specific routing using curl commands:

```bash
# Test tenant1 routing (goes to tenant1-backend)
curl -H "X-Tenant-ID: tenant1" http://localhost:8080/test

# Test tenant2 routing (goes to tenant2-backend)  
curl -H "X-Tenant-ID: tenant2" http://localhost:8080/test

# Test without tenant header (goes to global-default)
curl http://localhost:8080/test

# Test with unknown tenant (falls back to global-default)
curl -H "X-Tenant-ID: unknown" http://localhost:8080/test
```

Or run the comprehensive test script:
```bash
./test-tenant-routing.sh
```

## Configuration

The reverse proxy is configured through `config.yaml`:

### Reverse Proxy Configuration
```yaml
reverseproxy:
  backend_services:
    global-default: "http://localhost:9001"
    tenant1-backend: "http://localhost:9002"
    tenant2-backend: "http://localhost:9003"
    specific-api: "http://localhost:9004"
  default_backend: "global-default"
  tenant_id_header: "X-Tenant-ID"
  require_tenant_id: false
```

### ChiMux Configuration
```yaml
chimux:
  basepath: ""
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["Content-Type", "Authorization"]
  allow_credentials: false
  max_age: 300
```

### HTTP Server Configuration
```yaml
httpserver:
  host: "localhost"
  port: 8080
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 120
```

## How Tenant-Specific Routing Works

1. **Tenant Registration**: Tenants are registered programmatically with their specific configurations:
   ```go
   tenantService.RegisterTenant("tenant1", map[string]modular.ConfigProvider{
       "reverseproxy": modular.NewStdConfigProvider(&reverseproxy.ReverseProxyConfig{
           DefaultBackend: "tenant1-backend",
           BackendServices: map[string]string{
               "tenant1-backend": "http://localhost:9002",
           },
       }),
   })
   ```

2. **Request Processing**: When a request comes in:
   - The reverse proxy checks for the `X-Tenant-ID` header
   - If found and the tenant exists, it uses the tenant's default backend
   - If no tenant header or unknown tenant, it falls back to the global default backend

3. **Backend Selection**:
   - `tenant1` → `tenant1-backend` (port 9002)
   - `tenant2` → `tenant2-backend` (port 9003)
   - No tenant/unknown → `global-default` (port 9001)

## Key Modules Used

1. **ChiMux Module**: Provides HTTP routing with Chi router and CORS middleware
2. **ReverseProxy Module**: Handles request proxying to backend services with tenant awareness
3. **HTTPServer Module**: Manages the HTTP server lifecycle

## Use Cases

This example is perfect for:
- **Multi-tenant API gateways**: Different tenants can have different backend services
- **Tenant isolation**: Route tenant traffic to dedicated backend services
- **Migration scenarios**: Gradually move tenants to new backend services
- **A/B testing**: Route different tenant groups to different service versions
- **Load balancing**: Distribute tenants across different backend clusters
- **Legacy system modernization**: Proxy requests while maintaining tenant-specific routing

## Architecture

```
Client Request → Tenant ID Check → ChiMux Router → ReverseProxy → Tenant-Specific Backend
                       ↓              ↓
                 X-Tenant-ID     CORS Middleware
                    Header            
                       ↓
               Tenant Configuration
                 (if available)
                       ↓
              Tenant Default Backend
                    OR
               Global Default Backend
```

The request flow:
1. Extracts tenant ID from `X-Tenant-ID` header
2. Applies CORS middleware
3. Determines appropriate backend based on tenant configuration
4. Proxies request to the selected backend service
