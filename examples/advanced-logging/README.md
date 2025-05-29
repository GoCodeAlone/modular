# Advanced Logging Example

This example demonstrates the advanced logging capabilities of the HTTP client module, including detailed request/response logging, file-based logging, and comprehensive debugging features.

## What it demonstrates

- **Advanced HTTP Client Logging**: Detailed request and response logging
- **File-Based Logging**: Logging HTTP transactions to organized file structures
- **Header and Body Logging**: Comprehensive logging of HTTP headers and request/response bodies
- **Configurable Log Levels**: Control over what gets logged and log size limits
- **Integration with Reverse Proxy**: Logging HTTP client activity in a real-world scenario
- **Structured Log Organization**: Organized log files for requests, responses, and transactions

## Features

- Comprehensive HTTP request/response logging
- File-based logging with organized directory structure
- Configurable body logging with size limits
- Header logging for debugging
- Transaction correlation across request/response pairs
- Integration with reverse proxy for real HTTP traffic
- Graceful shutdown with log cleanup

## Running the Example

```bash
cd examples/advanced-logging

# Build the application
go build -o advanced-logging .

# Run the application
./advanced-logging
```

The application will:
1. Start a reverse proxy server on `localhost:8080`
2. Make test requests to demonstrate logging
3. Create detailed log files in the `./logs` directory
4. Run for 30 seconds to allow manual testing

## Configuration

### Advanced HTTP Client Logging Configuration
```yaml
httpclient:
  # Enable detailed verbose logging
  verbose: true
  
  # Advanced verbose logging options
  verbose_options:
    log_headers: true           # Log request and response headers
    log_body: true             # Log request and response bodies
    max_body_log_size: 5120    # Maximum size of logged bodies (5KB)
    log_to_file: true          # Log to files instead of just application logger
    log_file_path: "./logs"    # Directory for log files
```

### Reverse Proxy Setup
The example includes a reverse proxy configuration to generate real HTTP traffic for logging demonstration:

```yaml
reverseproxy:
  backend_services:
    httpbin: "https://httpbin.org"
  routes:
    "/proxy/httpbin": "httpbin"
    "/proxy/httpbin/*": "httpbin"
  default_backend: "httpbin"
```

## Testing the Logging

Once running, you can test the logging functionality:

```bash
# These requests will be logged in detail
curl http://localhost:8080/proxy/httpbin/json
curl http://localhost:8080/proxy/httpbin/headers
curl http://localhost:8080/proxy/httpbin/user-agent
curl http://localhost:8080/proxy/httpbin/get?param=value
```

## Log File Structure

The logging creates an organized directory structure:

```
logs/
├── requests/       # Individual request log files
├── responses/      # Individual response log files
└── transactions/   # Complete request-response transaction logs
```

Each log file contains:
- **Request logs**: HTTP method, URL, headers, body content
- **Response logs**: Status code, headers, response body
- **Transaction logs**: Complete request-response pairs with timing

## Log Content Examples

### Request Log
```
Timestamp: 2024-01-01T12:00:00Z
Method: GET
URL: https://httpbin.org/json
Headers:
  User-Agent: Go-http-client/1.1
  Accept-Encoding: gzip
Body: [empty]
```

### Response Log
```
Timestamp: 2024-01-01T12:00:01Z
Status: 200 OK
Headers:
  Content-Type: application/json
  Content-Length: 256
Body: {"key": "value", ...}
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `log_headers` | Log HTTP headers | `false` |
| `log_body` | Log request/response bodies | `false` |
| `max_body_log_size` | Maximum body size to log (bytes) | `1024` |
| `log_to_file` | Enable file logging | `false` |
| `log_file_path` | Directory for log files | `./logs` |

## Use Cases

This example is perfect for:
- Debugging HTTP client issues
- Monitoring API interactions
- Security auditing of HTTP traffic
- Performance analysis of HTTP requests
- Compliance logging requirements
- Development and testing scenarios

## Security Considerations

When using advanced logging in production:
- Be careful with body logging for sensitive data
- Set appropriate `max_body_log_size` limits
- Ensure log files are properly secured
- Consider log rotation and cleanup policies
- Review logged headers for sensitive information

The advanced logging feature provides comprehensive visibility into HTTP client behavior while maintaining configurability for different use cases and security requirements.
