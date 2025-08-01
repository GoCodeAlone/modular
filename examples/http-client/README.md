# HTTP Client Example

This example demonstrates the integration of the `httpclient` and `reverseproxy` modules, showcasing how the reverseproxy module properly uses the httpclient service for making HTTP requests with verbose logging.

## Features Demonstrated

- **Service Integration**: Shows how the reverseproxy module automatically uses the httpclient service when available
- **Verbose HTTP Logging**: Demonstrates detailed request/response logging through the httpclient service
- **File Logging**: Captures HTTP request/response details to files for analysis
- **Modular Architecture**: Clean separation of concerns between routing (reverseproxy) and HTTP client functionality (httpclient)
- **Service Dependency Resolution**: Example of how modules can depend on services provided by other modules

## What it demonstrates

- **HTTP Client Module Integration**: How the httpclient module integrates with other modules
- **Advanced HTTP Client Configuration**: Connection pooling, timeouts, and performance tuning
- **Reverse Proxy with Custom Client**: Using a configured HTTP client for proxying requests
- **Module Service Dependencies**: How modules can provide services to other modules
- **Verbose Logging Options**: Advanced HTTP client logging capabilities with file output

## Features

- Configured HTTP client with connection pooling
- Custom timeout settings for different operations
- Integration with reverse proxy for backend requests
- ChiMux router with CORS support
- HTTP server for receiving requests
- Compression and keep-alive settings
- **NEW**: Comprehensive HTTP request/response logging to files

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
  max_idle_conns: 100
  max_idle_conns_per_host: 10
  idle_conn_timeout: 90
  
  # Timeout settings
  request_timeout: 30
  tls_timeout: 10
  
  # Other settings
  disable_compression: false
  disable_keep_alives: false
  verbose: true
  
  # Verbose logging options (enable for demonstration)
  verbose_options:
    log_headers: true
    log_body: true
    max_body_log_size: 2048
    log_to_file: true
    log_file_path: "./http_client_logs"
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

## Verification

When the example runs correctly, you should see:

1. **Service Integration Success**: Log message showing `"Using HTTP client from httpclient service"` instead of `"Using default HTTP client (no httpclient service available)"`
2. **Verbose Logging**: Detailed HTTP request/response logs including timing information
3. **File Logging**: HTTP transaction logs saved to the `./http_client_logs` directory

## Key Features Demonstrated

1. **Connection Pooling**: Efficient reuse of HTTP connections
2. **Timeout Management**: Separate timeouts for requests and TLS handshakes
3. **Performance Tuning**: Optimized settings for high-throughput scenarios
4. **Compression Handling**: Configurable request/response compression
5. **Keep-Alive Control**: Connection persistence management
6. **Verbose Logging**: Request/response logging for debugging
7. **File-Based Logging**: Persistent HTTP transaction logs for analysis

## Module Architecture

```
HTTP Request → ChiMux Router → ReverseProxy Module → HTTP Client Module → Backend Service
                                        ↓
                            Uses configured HTTP client with:
                            - Connection pooling
                            - Custom timeouts
                            - Logging capabilities
                            - File-based transaction logs
```

## Use Cases

This example is ideal for:
- High-performance reverse proxies
- API gateways requiring connection optimization
- Services needing detailed HTTP client monitoring
- Applications with strict timeout requirements
- Systems requiring HTTP client telemetry
- Debugging and troubleshooting HTTP integrations

The HTTP client module provides enterprise-grade HTTP client functionality that can be shared across multiple modules in your application.
