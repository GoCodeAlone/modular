# Health-Aware Reverse Proxy Example

This example demonstrates a comprehensive health-aware reverse proxy setup using the modular framework. It showcases advanced health checking, circuit breaker patterns, and how to expose health endpoints for internal service monitoring.

## Features Demonstrated

### Health Checking
- **Comprehensive Backend Monitoring**: Health checks for all configured backends
- **Configurable Check Intervals**: Different health check intervals per backend
- **Smart Scheduling**: Skips health checks if recent requests have occurred
- **DNS Resolution Monitoring**: Tracks DNS resolution status for each backend
- **HTTP Connectivity Testing**: Tests actual HTTP connectivity with configurable timeouts
- **Custom Health Endpoints**: Support for custom health check endpoints per backend

### Circuit Breaker Integration
- **Automatic Failure Detection**: Circuit breakers automatically detect failing backends
- **Per-Backend Configuration**: Different circuit breaker settings per backend
- **Health Status Integration**: Circuit breaker status is included in health reports
- **Configurable Thresholds**: Customizable failure thresholds and recovery timeouts

### Health Endpoints
- **Overall Service Health**: `/health` endpoint that reflects overall service status
- **Detailed Backend Health**: `/metrics/reverseproxy/health` endpoint with detailed backend information
- **Proper HTTP Status Codes**: Returns 200 for healthy, 503 for unhealthy services
- **JSON Response Format**: Structured JSON responses with comprehensive status information

## Backend Services

The example starts several mock backend services to demonstrate different scenarios:

### 1. Healthy API (port 9001)
- **Status**: Always healthy and responsive
- **Health Check**: `/health` endpoint always returns 200
- **Circuit Breaker**: Configured with standard settings
- **Use Case**: Represents a reliable, well-functioning service

### 2. Intermittent API (port 9002)
- **Status**: Fails every 3rd request (simulates intermittent issues)
- **Health Check**: Health endpoint is always available
- **Circuit Breaker**: More sensitive settings (2 failures trigger circuit open)
- **Use Case**: Represents a service with reliability issues

### 3. Slow API (port 9003)
- **Status**: Always successful but with 2-second delay
- **Health Check**: Health endpoint responds without delay
- **Circuit Breaker**: Less sensitive settings (5 failures trigger circuit open)
- **Use Case**: Represents a slow but reliable service

### 4. Unreachable API (port 9999)
- **Status**: Service is not started (connection refused)
- **Health Check**: Will fail DNS/connectivity tests
- **Circuit Breaker**: Very sensitive settings (1 failure triggers circuit open)
- **Use Case**: Represents an unreachable or down service

## Configuration Features

### Health Check Configuration
```yaml
health_check:
  enabled: true
  interval: "10s"                    # Global check interval
  timeout: "3s"                      # Global timeout
  recent_request_threshold: "30s"    # Skip checks if recent traffic
  expected_status_codes: [200, 204]  # Expected healthy status codes
  
  # Per-backend overrides
  backend_health_check_config:
    healthy-api:
      interval: "5s"                 # More frequent checks
      timeout: "2s"
```

### Circuit Breaker Configuration
```yaml
circuit_breaker_config:
  enabled: true
  failure_threshold: 3               # Global failure threshold
  open_timeout: "30s"                # Global recovery timeout
  
# Per-backend overrides
backend_circuit_breakers:
  intermittent-api:
    failure_threshold: 2             # More sensitive
    open_timeout: "15s"              # Faster recovery
```

## Running the Example

1. **Start the application**:
   ```bash
   cd examples/health-aware-reverse-proxy
   go run main.go
   ```

2. **Test the backends**:
   ```bash
   # Test healthy API
   curl http://localhost:8080/api/healthy
   
   # Test intermittent API (may fail on every 3rd request)
   curl http://localhost:8080/api/intermittent
   
   # Test slow API (will take 2+ seconds)
   curl http://localhost:8080/api/slow
   
   # Test unreachable API (will fail immediately)
   curl http://localhost:8080/api/unreachable
   ```

3. **Check overall service health**:
   ```bash
   # Overall health status (suitable for load balancer health checks)
   curl http://localhost:8080/health
   
   # Detailed health information
   curl http://localhost:8080/metrics/reverseproxy/health
   ```

## Health Response Format

### Overall Health Endpoint (`/health`)
```json
{
  "healthy": true,
  "total_backends": 4,
  "healthy_backends": 3,
  "unhealthy_backends": 1,
  "circuit_open_count": 1,
  "last_check": "2024-01-01T12:00:00Z"
}
```

### Detailed Health Endpoint (`/metrics/reverseproxy/health`)
```json
{
  "healthy": true,
  "total_backends": 4,
  "healthy_backends": 3,
  "unhealthy_backends": 1,
  "circuit_open_count": 1,
  "last_check": "2024-01-01T12:00:00Z",
  "backend_details": {
    "healthy-api": {
      "backend_id": "healthy-api",
      "url": "http://localhost:9001",
      "healthy": true,
      "last_check": "2024-01-01T12:00:00Z",
      "last_success": "2024-01-01T12:00:00Z",
      "response_time": "15ms",
      "dns_resolved": true,
      "resolved_ips": ["127.0.0.1"],
      "circuit_breaker_open": false,
      "circuit_breaker_state": "closed",
      "circuit_failure_count": 0
    }
  }
}
```

## Use Cases

### 1. Load Balancer Health Checks
Use the `/health` endpoint for load balancer health checks. The endpoint returns:
- **HTTP 200**: Service is healthy (all backends operational)
- **HTTP 503**: Service is unhealthy (one or more backends down)

### 2. Internal Monitoring
Use the detailed health endpoint (`/metrics/reverseproxy/health`) for internal monitoring systems that need comprehensive backend status information.

### 3. Circuit Breaker Monitoring
Monitor circuit breaker status through the health endpoints to understand which services are experiencing issues and how the system is protecting itself.

### 4. Performance Monitoring
Track response times and success rates for each backend service through the health status information.

## Key Benefits

1. **Proactive Monitoring**: Health checks run continuously in the background
2. **Circuit Protection**: Automatic protection against cascading failures
3. **Comprehensive Status**: Full visibility into backend service health
4. **Configurable Sensitivity**: Different monitoring strategies per service type
5. **Standard Endpoints**: Health endpoints suitable for container orchestration platforms
6. **Operational Visibility**: Detailed information for troubleshooting and monitoring