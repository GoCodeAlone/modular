# Testing Scenarios Example

This example demonstrates comprehensive testing scenarios for reverse proxy and API gateway functionality using the modular framework. It supports all common testing patterns needed for production-ready API gateway systems, including **LaunchDarkly integration, debug endpoints, and dry-run functionality** as described in the Chimera Facade SCENARIOS.md file.

## Supported Testing Scenarios

### Core Testing Scenarios

### 1. Health Check Testing ✅
- Backend availability monitoring
- Custom health endpoints per backend  
- DNS resolution testing
- HTTP connectivity testing
- Configurable health check intervals and timeouts

### 2. Load Testing ✅
- High-concurrency request handling
- Connection pooling validation
- Resource utilization monitoring
- Performance baseline establishment

### 3. Failover/Circuit Breaker Testing ✅
- Backend failure simulation
- Circuit breaker state transitions
- Fallback behavior validation
- Recovery time testing

### 4. Feature Flag Testing ✅
- A/B deployment testing
- Gradual rollout scenarios
- Tenant-specific feature flags
- Dynamic routing based on flags

### 5. Multi-Tenant Testing ✅
- Tenant isolation validation
- Tenant-specific routing
- Cross-tenant security testing
- Configuration isolation

### 6. Security Testing ✅
- Authentication testing
- Authorization validation
- Rate limiting testing
- Header security validation

### 7. Performance Testing ✅
- Latency measurement
- Throughput testing
- Response time validation
- Caching effectiveness

### 8. Configuration Testing ✅
- Dynamic configuration updates
- Configuration validation
- Environment-specific configs
- Hot reloading validation

### 9. Error Handling Testing ✅
- Error propagation testing
- Custom error responses
- Retry mechanism testing
- Graceful degradation

### 10. Monitoring/Metrics Testing ✅
- Metrics collection validation
- Log aggregation testing
- Performance metrics
- Health status reporting

### Chimera Facade Scenarios (NEW)

Based on the Chimera Facade SCENARIOS.md file, the following specific scenarios are now supported:

### 11. Toolkit API with Feature Flag Control ✅
- Tests the `/api/v1/toolkit/toolbox` endpoint
- LaunchDarkly feature flag evaluation
- Tenant-specific configuration fallbacks
- Graceful degradation when LaunchDarkly is unavailable

### 12. OAuth Token API Testing ✅
- Tests the `/api/v1/authentication/oauth/token` endpoint
- Feature flag-controlled routing between Chimera and tenant backends
- Tenant-specific configuration support

### 13. OAuth Introspection API Testing ✅
- Tests the `/api/v1/authentication/oauth/introspect` endpoint
- Feature flag-controlled routing
- POST method validation

### 14. Tenant Configuration Loading ✅
- Per-tenant configuration loading from separate YAML files
- Feature flag fallback behavior
- Support for `sampleaff1` and other tenant configurations

### 15. Debug and Monitoring Endpoints ✅
- `/debug/flags` - Feature flag status and evaluation
- `/debug/info` - General system information
- `/debug/backends` - Backend status and configuration
- `/debug/circuit-breakers` - Circuit breaker states
- `/debug/health-checks` - Health check status

### 16. Dry-Run Testing ✅
- Tests the `/api/v1/test/dryrun` endpoint
- Sends requests to both primary and alternative backends
- Compares responses and logs differences
- Configurable header comparison and filtering

## LaunchDarkly Integration

### Features
- **LaunchDarkly SDK Integration**: Placeholder implementation ready for actual SDK integration
- **Feature Flag Evaluation**: Real-time evaluation with tenant context
- **Graceful Degradation**: Falls back to tenant config when LaunchDarkly unavailable
- **Debug Endpoint**: `/debug/flags` for debugging feature flag status
- **Tenant Context**: Uses `X-Affiliate-ID` header for tenant-specific flag evaluation

### Configuration
```yaml
reverseproxy:
  launchdarkly:
    sdk_key: ""  # Set via LAUNCHDARKLY_SDK_KEY environment variable
    environment: "local"
    timeout: "5s"
    offline: false
```

### Environment Setup
```bash
export LAUNCHDARKLY_SDK_KEY=sdk-key-your-launchdarkly-key-here
export LAUNCHDARKLY_ENVIRONMENT=local
```

## Quick Start

```bash
cd examples/testing-scenarios

# Build the application
go build -o testing-scenarios .

# Run demonstration of all key scenarios (recommended first run)
./demo.sh

# Run comprehensive Chimera Facade scenarios
./test-chimera-scenarios.sh

# Run with basic configuration
./testing-scenarios

# Run specific test scenario
./testing-scenarios --scenario toolkit-api
./testing-scenarios --scenario oauth-token
./testing-scenarios --scenario debug-endpoints
./testing-scenarios --scenario dry-run
```

## Individual Scenario Testing

Each scenario can be run independently for focused testing:

```bash
# Chimera Facade specific scenarios
./testing-scenarios --scenario=toolkit-api --duration=60s
./testing-scenarios --scenario=oauth-token --duration=60s
./testing-scenarios --scenario=oauth-introspect --duration=60s
./testing-scenarios --scenario=tenant-config --duration=60s
./testing-scenarios --scenario=debug-endpoints --duration=60s
./testing-scenarios --scenario=dry-run --duration=60s

# Original testing scenarios
./testing-scenarios --scenario=health-check --duration=60s
./testing-scenarios --scenario=load-test --connections=100 --duration=120s
./testing-scenarios --scenario=failover --backend=primary --failure-rate=0.5
./testing-scenarios --scenario=feature-flags --tenant=test-tenant --flag=new-api
./testing-scenarios --scenario=performance --metrics=detailed --export=json
```

## Automated Test Scripts

Each scenario includes automated test scripts:

- `demo.sh` - **Quick demonstration of all key scenarios including Chimera Facade**
- `test-chimera-scenarios.sh` - **Comprehensive Chimera Facade scenario testing**
- `test-all.sh` - Comprehensive test suite for all scenarios
- `test-health-checks.sh` - Health check scenarios
- `test-load.sh` - Load testing scenarios  
- `test-feature-flags.sh` - Feature flag scenarios

### Running Automated Tests

```bash
# Quick demonstration (recommended first run)
./demo.sh

# Comprehensive Chimera Facade testing
./test-chimera-scenarios.sh

# Comprehensive testing
./test-all.sh

# Specific scenario testing
./test-health-checks.sh
./test-load.sh --requests 200 --concurrency 20
./test-feature-flags.sh

# All tests with custom parameters
./test-all.sh --verbose --timeout 10
```

## Configuration

The example uses `config.yaml` for comprehensive configuration covering all testing scenarios:

```yaml
reverseproxy:
  # Multiple backend services for different test scenarios
  backend_services:
    primary: "http://localhost:9001"
    secondary: "http://localhost:9002"
    canary: "http://localhost:9003"
    legacy: "http://localhost:9004"
    monitoring: "http://localhost:9005"
    unstable: "http://localhost:9006"    # For circuit breaker testing
    slow: "http://localhost:9007"        # For performance testing
    chimera: "http://localhost:9008"     # For Chimera API scenarios
  
  # Route-level feature flag configuration for LaunchDarkly scenarios
  route_configs:
    "/api/v1/toolkit/toolbox":
      feature_flag_id: "toolkit-toolbox-api"
      alternative_backend: "legacy"
    "/api/v1/authentication/oauth/token":
      feature_flag_id: "oauth-token-api"
      alternative_backend: "legacy"
    "/api/v1/authentication/oauth/introspect":
      feature_flag_id: "oauth-introspect-api"
      alternative_backend: "legacy"
    "/api/v1/test/dryrun":
      feature_flag_id: "test-dryrun-api"
      alternative_backend: "legacy"
      dry_run: true
      dry_run_backend: "chimera"
  
  # LaunchDarkly integration
  launchdarkly:
    sdk_key: ""  # Set via environment variable
    environment: "local"
    timeout: "5s"
    
  # Debug endpoints
  debug_endpoints:
    enabled: true
    base_path: "/debug"
    require_auth: false
    
  # Dry-run configuration
  dry_run:
    enabled: true
    log_responses: true
    max_response_size: 1048576  # 1MB
    compare_headers: ["Content-Type", "X-API-Version"]
    ignore_headers: ["Date", "X-Request-ID", "X-Trace-ID"]
  
  # Multi-tenant configuration with X-Affiliate-ID header
  tenant_id_header: "X-Affiliate-ID"
  require_tenant_id: false
```

## Architecture

```
Client → Testing Proxy → Feature Flag Evaluator → Backend Pool
           ↓                  ↓                      ↓
      Debug Endpoints    LaunchDarkly/Config    Health Checks
           ↓                  ↓                      ↓
      Dry-Run Handler    Circuit Breaker        Load Balancer
```

## Mock Backend System

The application automatically starts 8 mock backends:

- **Primary** (port 9001): Main backend for standard testing
- **Secondary** (port 9002): Secondary backend for failover testing
- **Canary** (port 9003): Canary backend for feature flag testing
- **Legacy** (port 9004): Legacy backend with `/status` endpoint
- **Monitoring** (port 9005): Monitoring backend with metrics
- **Unstable** (port 9006): Unstable backend for circuit breaker testing
- **Slow** (port 9007): Slow backend for performance testing
- **Chimera** (port 9008): Chimera API backend for LaunchDarkly scenarios

Each backend can be configured with:
- Custom failure rates
- Response delays
- Different health endpoints
- Request counting and metrics
- Specific API endpoints (Chimera/Legacy)

## Testing Features

### Health Check Testing
- Tests all backend health endpoints
- Validates health check routing through proxy
- Tests tenant-specific health checks
- Monitors health check stability over time

### Load Testing
- Sequential and concurrent request testing
- Configurable request counts and concurrency
- Response time measurement
- Success rate calculation
- Throughput measurement

### Failover Testing  
- Simulates backend failures
- Tests circuit breaker behavior
- Validates fallback mechanisms
- Tests recovery scenarios

### Feature Flag Testing
- Tests enabled/disabled routing
- Tenant-specific feature flags
- Dynamic flag changes
- Fallback behavior validation
- LaunchDarkly integration testing

### Multi-Tenant Testing
- Tenant isolation validation
- Tenant-specific routing using `X-Affiliate-ID` header
- Concurrent tenant testing
- Default behavior testing
- Support for `sampleaff1` and other tenants

### Debug Endpoints Testing
- Feature flag status debugging
- System information retrieval
- Backend status monitoring
- Circuit breaker state inspection
- Health check status verification

### Dry-Run Testing
- Concurrent requests to multiple backends
- Response comparison and difference analysis
- Configurable header filtering
- Comprehensive logging of results

## Production Readiness Validation

This example validates:
- ✅ High availability configurations
- ✅ Performance characteristics and bottlenecks
- ✅ Security posture and threat response
- ✅ Monitoring and observability capabilities
- ✅ Multi-tenant isolation and routing
- ✅ Feature rollout and deployment strategies
- ✅ Error handling and recovery mechanisms
- ✅ Circuit breaker and failover behavior
- ✅ LaunchDarkly integration and graceful degradation
- ✅ Debug capabilities for troubleshooting
- ✅ Dry-run functionality for safe testing

## Use Cases

Perfect for validating:
- **API Gateway Deployments**: Ensure production readiness
- **Performance Tuning**: Identify bottlenecks and optimize settings
- **Resilience Testing**: Validate failure handling and recovery
- **Multi-Tenant Systems**: Ensure proper isolation and routing
- **Feature Rollouts**: Test gradual deployment strategies with LaunchDarkly
- **Monitoring Setup**: Validate observability and alerting
- **Chimera Facade Integration**: Test all scenarios from SCENARIOS.md
- **Debug and Troubleshooting**: Validate debug endpoint functionality
- **Dry-Run Deployments**: Safe testing of new backends

## Chimera Facade Specific Testing

This implementation covers all scenarios described in the Chimera Facade SCENARIOS.md file:

### Endpoints Tested
- ✅ **Health Check**: `/health` endpoint accessibility
- ✅ **Toolkit API**: `/api/v1/toolkit/toolbox` with feature flag control
- ✅ **OAuth Token**: `/api/v1/authentication/oauth/token` with routing
- ✅ **OAuth Introspection**: `/api/v1/authentication/oauth/introspect` with routing
- ✅ **Debug Endpoints**: `/debug/flags`, `/debug/info`, etc.
- ✅ **Dry-Run Endpoint**: `/api/v1/test/dryrun` for backend comparison

### Features Validated
- ✅ **LaunchDarkly Integration**: Feature flag evaluation with tenant context
- ✅ **Graceful Degradation**: Fallback to tenant config when LaunchDarkly unavailable
- ✅ **Tenant Configuration**: Per-tenant feature flag configuration
- ✅ **Debug Capabilities**: Comprehensive debug endpoints for troubleshooting
- ✅ **Dry-Run Mode**: Backend response comparison and logging
- ✅ **Multi-Tenant Routing**: Support for `X-Affiliate-ID` header

## Example Output

```bash
$ ./demo.sh
╔══════════════════════════════════════════════════════════════╗
║           Testing Scenarios Demonstration                   ║
║     Including Chimera Facade LaunchDarkly Integration       ║
╚══════════════════════════════════════════════════════════════╝

Test 1: Health Check Scenarios
  General health check... ✓ PASS
  API v1 health... ✓ PASS
  Legacy health... ✓ PASS

Test 2: Chimera Facade Scenarios
  Toolkit API... ✓ PASS
  OAuth Token API... ✓ PASS
  OAuth Introspection API... ✓ PASS

Test 3: Multi-Tenant Scenarios  
  Alpha tenant... ✓ PASS
  Beta tenant... ✓ PASS
  SampleAff1 tenant... ✓ PASS
  No tenant (default)... ✓ PASS

Test 4: Debug and Monitoring Endpoints
  Feature flags debug... ✓ PASS
  System debug info... ✓ PASS
  Backend status... ✓ PASS

Test 5: Dry-Run Testing
  Dry-run GET request... ✓ PASS
  Dry-run POST request... ✓ PASS

✓ All scenarios completed successfully
```

This comprehensive testing example ensures that your reverse proxy configuration is production-ready and handles all common operational scenarios, including the specific Chimera Facade requirements with LaunchDarkly integration, debug endpoints, and dry-run functionality.