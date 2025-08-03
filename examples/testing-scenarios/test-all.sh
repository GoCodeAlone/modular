#!/bin/bash

# Comprehensive Testing Scenarios Script
# Tests all reverse proxy and API gateway scenarios

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROXY_URL="http://localhost:8080"
TIMEOUT=30
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --timeout|-t)
            TIMEOUT="$2"
            shift 2
            ;;
        --url|-u)
            PROXY_URL="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --verbose, -v     Enable verbose output"
            echo "  --timeout, -t     Set request timeout (default: 30)"
            echo "  --url, -u         Set proxy URL (default: http://localhost:8080)"
            echo "  --help, -h        Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Helper functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

test_request() {
    local description="$1"
    local method="${2:-GET}"
    local path="${3:-/}"
    local headers="${4:-}"
    local data="${5:-}"
    local expected_status="${6:-200}"
    
    echo -n "  Testing: $description... "
    
    local cmd="curl -s -w '%{http_code}' -m $TIMEOUT -X $method"
    
    if [[ -n "$headers" ]]; then
        while IFS= read -r header; do
            if [[ -n "$header" ]]; then
                cmd="$cmd -H '$header'"
            fi
        done <<< "$headers"
    fi
    
    if [[ -n "$data" ]]; then
        cmd="$cmd -d '$data'"
    fi
    
    cmd="$cmd '$PROXY_URL$path'"
    
    if [[ "$VERBOSE" == "true" ]]; then
        echo
        echo "    Command: $cmd"
    fi
    
    local response
    response=$(eval "$cmd" 2>/dev/null) || {
        error "Request failed"
        return 1
    }
    
    local status_code="${response: -3}"
    local body="${response%???}"
    
    if [[ "$status_code" == "$expected_status" ]]; then
        success "HTTP $status_code"
        if [[ "$VERBOSE" == "true" && -n "$body" ]]; then
            echo "    Response: $body"
        fi
        return 0
    else
        error "Expected HTTP $expected_status, got HTTP $status_code"
        if [[ -n "$body" ]]; then
            echo "    Response: $body"
        fi
        return 1
    fi
}

wait_for_service() {
    local service_url="$1"
    local max_attempts="${2:-30}"
    local attempt=1
    
    echo -n "  Waiting for service at $service_url... "
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s -f "$service_url" >/dev/null 2>&1; then
            success "Service ready (attempt $attempt)"
            return 0
        fi
        
        sleep 1
        ((attempt++))
    done
    
    error "Service not ready after $max_attempts attempts"
    return 1
}

run_health_check_tests() {
    echo -e "${PURPLE}=== Health Check Testing Scenarios ===${NC}"
    
    # Test basic health endpoint
    test_request "Basic health check" "GET" "/health"
    
    # Test backend-specific health checks
    test_request "Primary backend health" "GET" "/api/v1/health"
    test_request "Secondary backend health" "GET" "/api/v2/health"
    test_request "Legacy backend health" "GET" "/legacy/status"
    
    # Test health check with different methods
    test_request "Health check with POST" "POST" "/health"
    test_request "Health check with OPTIONS" "OPTIONS" "/health"
    
    echo
}

run_load_testing_scenarios() {
    echo -e "${PURPLE}=== Load Testing Scenarios ===${NC}"
    
    # Sequential load test
    echo "  Running sequential load test (10 requests)..."
    local success_count=0
    for i in {1..10}; do
        if test_request "Load test request $i" "GET" "/api/v1/test" "" "" "200" >/dev/null 2>&1; then
            ((success_count++))
        fi
    done
    info "Sequential load test: $success_count/10 requests successful"
    
    # Concurrent load test (using background processes)
    echo "  Running concurrent load test (5 parallel requests)..."
    local pids=()
    for i in {1..5}; do
        (
            test_request "Concurrent request $i" "GET" "/api/v1/concurrent" "" "" "200" >/dev/null 2>&1
            echo $? > "/tmp/load_test_$i.result"
        ) &
        pids+=($!)
    done
    
    # Wait for all background jobs
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Count successful concurrent requests
    success_count=0
    for i in {1..5}; do
        if [[ -f "/tmp/load_test_$i.result" ]]; then
            if [[ $(cat "/tmp/load_test_$i.result") == "0" ]]; then
                ((success_count++))
            fi
            rm -f "/tmp/load_test_$i.result"
        fi
    done
    info "Concurrent load test: $success_count/5 requests successful"
    
    echo
}

run_failover_testing() {
    echo -e "${PURPLE}=== Failover/Circuit Breaker Testing ===${NC}"
    
    # Test normal operation
    test_request "Normal operation before failover" "GET" "/api/v1/test"
    
    # Test with unstable backend (this should trigger circuit breaker)
    warning "Testing unstable backend (may fail - this is expected)"
    test_request "Unstable backend test" "GET" "/unstable/test" "" "" "500"
    
    # Test fallback behavior
    test_request "Fallback after circuit breaker" "GET" "/api/v1/fallback"
    
    echo
}

run_feature_flag_testing() {
    echo -e "${PURPLE}=== Feature Flag Testing ===${NC}"
    
    # Test with feature flag headers
    test_request "Feature flag enabled" "GET" "/api/v1/test" "X-Feature-Flag: api-v1-enabled"
    test_request "Feature flag disabled" "GET" "/api/v2/test" "X-Feature-Flag: api-v2-disabled"
    
    # Test canary routing
    test_request "Canary feature test" "GET" "/api/canary/test" "X-Feature-Flag: canary-enabled"
    
    echo
}

run_multi_tenant_testing() {
    echo -e "${PURPLE}=== Multi-Tenant Testing ===${NC}"
    
    # Test different tenants
    test_request "Alpha tenant" "GET" "/api/v1/test" "X-Tenant-ID: tenant-alpha"
    test_request "Beta tenant" "GET" "/api/v1/test" "X-Tenant-ID: tenant-beta"
    test_request "Canary tenant" "GET" "/api/v1/test" "X-Tenant-ID: tenant-canary"
    test_request "Enterprise tenant" "GET" "/api/enterprise/test" "X-Tenant-ID: tenant-enterprise"
    
    # Test no tenant (should use default)
    test_request "No tenant (default)" "GET" "/api/v1/test"
    
    # Test unknown tenant (should use default)
    test_request "Unknown tenant" "GET" "/api/v1/test" "X-Tenant-ID: unknown-tenant"
    
    echo
}

run_security_testing() {
    echo -e "${PURPLE}=== Security Testing ===${NC}"
    
    # Test CORS headers
    test_request "CORS preflight" "OPTIONS" "/api/v1/test" "Origin: https://example.com"
    
    # Test with various security headers
    test_request "Request with auth header" "GET" "/api/v1/secure" "Authorization: Bearer test-token"
    test_request "Request without auth" "GET" "/api/v1/secure"
    
    # Test header injection prevention
    test_request "Header injection test" "GET" "/api/v1/test" "X-Malicious-Header: \r\nInjected: header"
    
    echo
}

run_performance_testing() {
    echo -e "${PURPLE}=== Performance Testing ===${NC}"
    
    # Test response times
    echo "  Measuring response times..."
    for endpoint in "/api/v1/fast" "/slow/test" "/api/v1/cached"; do
        echo -n "    Testing $endpoint... "
        local start_time=$(date +%s%N)
        if test_request "Performance test" "GET" "$endpoint" "" "" "200" >/dev/null 2>&1; then
            local end_time=$(date +%s%N)
            local duration=$(((end_time - start_time) / 1000000)) # Convert to milliseconds
            info "Response time: ${duration}ms"
        else
            error "Request failed"
        fi
    done
    
    echo
}

run_configuration_testing() {
    echo -e "${PURPLE}=== Configuration Testing ===${NC}"
    
    # Test different route configurations
    test_request "V1 API route" "GET" "/api/v1/config"
    test_request "V2 API route" "GET" "/api/v2/config"
    test_request "Legacy route" "GET" "/legacy/config"
    test_request "Monitoring route" "GET" "/metrics/config"
    
    # Test path rewriting
    test_request "Path rewriting test" "GET" "/api/v1/rewrite/test"
    
    echo
}

run_error_handling_testing() {
    echo -e "${PURPLE}=== Error Handling Testing ===${NC}"
    
    # Test various error conditions
    test_request "404 error test" "GET" "/nonexistent/endpoint" "" "" "404"
    test_request "Method not allowed" "TRACE" "/api/v1/test" "" "" "405"
    
    # Test error responses with specific backends
    warning "Testing error conditions (errors are expected)"
    test_request "Backend error test" "GET" "/unstable/error" "" "" "500"
    
    echo
}

run_monitoring_testing() {
    echo -e "${PURPLE}=== Monitoring/Metrics Testing ===${NC}"
    
    # Test metrics endpoints
    test_request "Application metrics" "GET" "/metrics"
    test_request "Reverse proxy metrics" "GET" "/reverseproxy/metrics"
    test_request "Backend monitoring" "GET" "/metrics/health"
    
    # Test logging and tracing
    test_request "Request with trace ID" "GET" "/api/v1/trace" "X-Trace-ID: test-trace-123"
    
    echo
}

# Main execution
main() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}         ${YELLOW}Comprehensive Reverse Proxy Testing Scenarios${NC}         ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo
    
    log "Starting comprehensive testing scenarios"
    log "Proxy URL: $PROXY_URL"
    log "Request timeout: ${TIMEOUT}s"
    log "Verbose mode: $VERBOSE"
    echo
    
    # Wait for the proxy service to be ready
    wait_for_service "$PROXY_URL/health" 60
    echo
    
    # Run all test scenarios
    local start_time=$(date +%s)
    
    run_health_check_tests
    run_load_testing_scenarios
    run_failover_testing
    run_feature_flag_testing
    run_multi_tenant_testing
    run_security_testing
    run_performance_testing
    run_configuration_testing
    run_error_handling_testing
    run_monitoring_testing
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}                    ${YELLOW}Testing Complete!${NC}                      ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}                                                              ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}  All reverse proxy testing scenarios completed successfully ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}  Total execution time: ${duration} seconds                        ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    
    log "All testing scenarios completed in ${duration} seconds"
}

# Run main function
main "$@"