# HTTP Client Example

This example demonstrates the integration of the HTTP client module with other modules in a reverse proxy setup, showcasing advanced HTTP client features and configuration.

## What it demonstrates

- **HTTP Client Module Integration**: How the httpclient module integrates with other modules
- **Advanced HTTP Client Configuration**: Connection pooling, timeouts, and performance tuning
- **Reverse Proxy with Custom Client**: Using a configured HTTP client for proxying requests
- **Module Service Dependencies**: How modules can provide services to other modules
- **Verbose Logging Options**: Basic HTTP client logging capabilities

## Features

- Configured HTTP client with connection pooling
- Custom timeout settings for different operations
- Integration with reverse proxy for backend requests
- ChiMux router with CORS support
- HTTP server for receiving requests
- Compression and keep-alive settings

## Running the Example

```bash
cd examples/http-client

# Build the application
go build -o http-client .

# Run the application
./http-client
```

The server will start on `localhost:8080` and act as a reverse proxy that uses the configured HTTP client for backend requests.

## Configuration

### HTTP Client Configuration
```yaml
httpclient:
  # Connection pooling settings
  max_idle_conns: 50
  max_idle_conns_per_host: 5
  idle_conn_timeout: 60
  
  # Timeout settings
  request_timeout: 15
  tls_timeout: 5
  
  # Other settings
  disable_compression: false
  disable_keep_alives: false
  verbose: true
  
  # Verbose logging options
  verbose_options:
    log_headers: false
    log_body: false
    max_body_log_size: 1024
```

### Reverse Proxy Integration
```yaml
reverseproxy:
  backend_services:
    httpbin: "https://httpbin.org"
  routes:
    "/proxy/httpbin": "httpbin"
    "/proxy/httpbin/*": "httpbin"
  default_backend: "httpbin"
```

## Testing the Integration

Once running, you can test the HTTP client integration:

```bash
# Test proxied requests (these use the configured HTTP client)
curl http://localhost:8080/proxy/httpbin/json
curl http://localhost:8080/proxy/httpbin/headers
curl http://localhost:8080/proxy/httpbin/user-agent
```

## Key Features Demonstrated

1. **Connection Pooling**: Efficient reuse of HTTP connections
2. **Timeout Management**: Separate timeouts for requests and TLS handshakes
3. **Performance Tuning**: Optimized settings for high-throughput scenarios
4. **Compression Handling**: Configurable request/response compression
5. **Keep-Alive Control**: Connection persistence management
6. **Verbose Logging**: Request/response logging for debugging

## Module Architecture

```
HTTP Request → ChiMux Router → ReverseProxy Module → HTTP Client Module → Backend Service
                                        ↓
                            Uses configured HTTP client with:
                            - Connection pooling
                            - Custom timeouts
                            - Logging capabilities
```

## Use Cases

This example is ideal for:
- High-performance reverse proxies
- API gateways requiring connection optimization
- Services needing detailed HTTP client monitoring
- Applications with strict timeout requirements
- Systems requiring HTTP client telemetry

The HTTP client module provides enterprise-grade HTTP client functionality that can be shared across multiple modules in your application.
