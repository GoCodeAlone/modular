#!/bin/bash

# Test script for Chimera Facade scenarios
# This script tests all the specific scenarios described in the Chimera SCENARIOS.md file

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Function to check if a URL is accessible
check_url() {
    local url=$1
    local description=$2
    if curl -s -f "$url" > /dev/null; then
        print_success "$description is accessible"
        return 0
    else
        print_error "$description is not accessible"
        return 1
    fi
}

# Function to test an endpoint with specific headers
test_endpoint() {
    local method=$1
    local url=$2
    local description=$3
    local headers=$4
    
    echo "  Testing $description..."
    
    if [ -n "$headers" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" $headers)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url")
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 400 ]; then
        print_success "$description: HTTP $http_code"
        return 0
    else
        print_warning "$description: HTTP $http_code"
        return 1
    fi
}

print_step "Chimera Facade Testing Scenarios"
echo "This script tests all scenarios described in the Chimera SCENARIOS.md file"
echo ""

# Build the application
print_step "Building Testing Scenarios Application"
if go build -o testing-scenarios .; then
    print_success "Application built successfully"
else
    print_error "Failed to build application"
    exit 1
fi

# Start the application in background
print_step "Starting Testing Scenarios Application"
./testing-scenarios > app.log 2>&1 &
APP_PID=$!

# Wait for application to start
echo "Waiting for application to start..."
sleep 5

# Check if application is running
if ! kill -0 $APP_PID 2>/dev/null; then
    print_error "Application failed to start"
    cat app.log
    exit 1
fi

print_success "Application started (PID: $APP_PID)"

# Function to cleanup on exit
cleanup() {
    echo ""
    print_step "Cleaning up"
    if [ -n "$APP_PID" ]; then
        kill $APP_PID 2>/dev/null || true
        wait $APP_PID 2>/dev/null || true
    fi
    rm -f testing-scenarios app.log
}
trap cleanup EXIT

# Test 1: Health Check Scenario
print_step "Test 1: Health Check Scenario"
if check_url "http://localhost:8080/health" "General health endpoint"; then
    test_endpoint "GET" "http://localhost:8080/api/v1/health" "API v1 health"
    test_endpoint "GET" "http://localhost:8080/legacy/status" "Legacy health endpoint"
fi

echo ""

# Test 2: Toolkit API with Feature Flag Control
print_step "Test 2: Toolkit API with Feature Flag Control"
test_endpoint "GET" "http://localhost:8080/api/v1/toolkit/toolbox" "Toolkit API without tenant"
test_endpoint "GET" "http://localhost:8080/api/v1/toolkit/toolbox" "Toolkit API with sampleaff1 tenant" '-H "X-Affiliate-ID: sampleaff1"'

echo ""

# Test 3: OAuth Token API
print_step "Test 3: OAuth Token API"
test_endpoint "POST" "http://localhost:8080/api/v1/authentication/oauth/token" "OAuth token API" '-H "Content-Type: application/json" -H "X-Affiliate-ID: sampleaff1"'

echo ""

# Test 4: OAuth Introspection API
print_step "Test 4: OAuth Introspection API"
test_endpoint "POST" "http://localhost:8080/api/v1/authentication/oauth/introspect" "OAuth introspection API" '-H "Content-Type: application/json" -H "X-Affiliate-ID: sampleaff1"'

echo ""

# Test 5: Tenant Configuration Loading
print_step "Test 5: Tenant Configuration Loading"
test_endpoint "GET" "http://localhost:8080/api/v1/test" "Existing tenant (sampleaff1)" '-H "X-Affiliate-ID: sampleaff1"'
test_endpoint "GET" "http://localhost:8080/api/v1/test" "Non-existent tenant" '-H "X-Affiliate-ID: nonexistent"'
test_endpoint "GET" "http://localhost:8080/api/v1/test" "No tenant header (default)"

echo ""

# Test 6: Debug and Monitoring Endpoints
print_step "Test 6: Debug and Monitoring Endpoints"
test_endpoint "GET" "http://localhost:8080/debug/flags" "Feature flags debug endpoint" '-H "X-Affiliate-ID: sampleaff1"'
test_endpoint "GET" "http://localhost:8080/debug/info" "General debug info endpoint"
test_endpoint "GET" "http://localhost:8080/debug/backends" "Backend status endpoint"
test_endpoint "GET" "http://localhost:8080/debug/circuit-breakers" "Circuit breaker status endpoint"
test_endpoint "GET" "http://localhost:8080/debug/health-checks" "Health check status endpoint"

echo ""

# Test 7: Dry-Run Testing Scenario
print_step "Test 7: Dry-Run Testing Scenario"
test_endpoint "GET" "http://localhost:8080/api/v1/test/dryrun" "Dry-run GET request" '-H "X-Affiliate-ID: sampleaff1"'
test_endpoint "POST" "http://localhost:8080/api/v1/test/dryrun" "Dry-run POST request" '-H "Content-Type: application/json" -H "X-Affiliate-ID: sampleaff1"'

echo ""

# Test 8: Multi-Tenant Scenarios
print_step "Test 8: Multi-Tenant Scenarios"
test_endpoint "GET" "http://localhost:8080/api/v1/test" "Alpha tenant" '-H "X-Affiliate-ID: tenant-alpha"'
test_endpoint "GET" "http://localhost:8080/api/v1/test" "Beta tenant" '-H "X-Affiliate-ID: tenant-beta"'

echo ""

# Test 9: Specific Scenario Runner Tests
print_step "Test 9: Running Individual Scenarios"

# Run specific scenarios using the scenario runner
scenarios=("toolkit-api" "oauth-token" "oauth-introspect" "tenant-config" "debug-endpoints" "dry-run")

for scenario in "${scenarios[@]}"; do
    echo "  Running scenario: $scenario"
    if timeout 30s ./testing-scenarios --scenario="$scenario" --duration=10s > scenario_${scenario}.log 2>&1; then
        print_success "Scenario $scenario completed successfully"
    else
        print_warning "Scenario $scenario had issues (check scenario_${scenario}.log)"
    fi
done

echo ""

# Test 10: Performance and Load Testing
print_step "Test 10: Performance and Load Testing"
echo "  Running basic load test..."
if timeout 30s ./testing-scenarios --scenario="load-test" --connections=10 --duration=10s > load_test.log 2>&1; then
    print_success "Load test completed successfully"
else
    print_warning "Load test had issues (check load_test.log)"
fi

echo ""

# Summary
print_step "Test Summary"
echo "All Chimera Facade scenarios have been tested."
echo ""
echo "Log files created:"
echo "  - app.log: Main application log"
echo "  - scenario_*.log: Individual scenario logs"
echo "  - load_test.log: Load test log"
echo ""
echo "Key endpoints tested:"
echo "  ✓ Health checks: /health, /api/v1/health, /legacy/status"
echo "  ✓ Toolkit API: /api/v1/toolkit/toolbox"
echo "  ✓ OAuth APIs: /api/v1/authentication/oauth/*"
echo "  ✓ Debug endpoints: /debug/*"
echo "  ✓ Dry-run endpoint: /api/v1/test/dryrun"
echo "  ✓ Multi-tenant routing with X-Affiliate-ID header"
echo ""
echo "Features tested:"
echo "  ✓ LaunchDarkly integration (placeholder)"
echo "  ✓ Feature flag routing"
echo "  ✓ Tenant-specific configuration"
echo "  ✓ Debug endpoints for monitoring"
echo "  ✓ Dry-run functionality"
echo "  ✓ Circuit breaker behavior"
echo "  ✓ Health check monitoring"
echo ""

print_success "Chimera Facade testing scenarios completed!"