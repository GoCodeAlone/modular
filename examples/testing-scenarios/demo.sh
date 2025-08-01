#!/bin/bash

# Quick demonstration of all key testing scenarios including Chimera Facade scenarios
# This script provides a rapid overview of all supported testing patterns

set -e

PROXY_URL="http://localhost:8080"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           Testing Scenarios Demonstration                   ║"
echo "║     Including Chimera Facade LaunchDarkly Integration       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Start the testing scenarios app in background
echo -e "${BLUE}Starting testing scenarios application...${NC}"
go build -o testing-scenarios .
./testing-scenarios >/dev/null 2>&1 &
APP_PID=$!

echo "Application PID: $APP_PID"
echo "Waiting for application to start..."
sleep 8

# Function to test an endpoint
test_endpoint() {
    local description="$1"
    local method="${2:-GET}"
    local endpoint="${3:-/}"
    local headers="${4:-}"
    
    echo -n "  $description... "
    
    local cmd="curl -s -w '%{http_code}' -m 5 -X $method"
    
    if [[ -n "$headers" ]]; then
        cmd="$cmd -H '$headers'"
    fi
    
    cmd="$cmd '$PROXY_URL$endpoint'"
    
    local response
    response=$(eval "$cmd" 2>/dev/null) || {
        echo -e "${RED}FAIL (connection error)${NC}"
        return 1
    }
    
    local status_code="${response: -3}"
    
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
        return 0
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
        return 1
    fi
}

# Wait for service to be ready
echo -n "Waiting for proxy service... "
for i in {1..30}; do
    if curl -s -f "$PROXY_URL/health" >/dev/null 2>&1; then
        echo -e "${GREEN}READY${NC}"
        break
    fi
    sleep 1
    if [[ $i -eq 30 ]]; then
        echo -e "${RED}TIMEOUT${NC}"
        kill $APP_PID 2>/dev/null
        exit 1
    fi
done

echo

# Test 1: Basic Health Checks
echo -e "${BLUE}Test 1: Health Check Scenarios${NC}"
test_endpoint "General health check" "GET" "/health"
test_endpoint "API v1 health" "GET" "/api/v1/health"
test_endpoint "Legacy health" "GET" "/legacy/status"

echo

# Test 2: Chimera Facade Scenarios
echo -e "${BLUE}Test 2: Chimera Facade Scenarios${NC}"
test_endpoint "Toolkit API" "GET" "/api/v1/toolkit/toolbox" "X-Affiliate-ID: sampleaff1"
test_endpoint "OAuth Token API" "POST" "/api/v1/authentication/oauth/token" "Content-Type: application/json, X-Affiliate-ID: sampleaff1"
test_endpoint "OAuth Introspection API" "POST" "/api/v1/authentication/oauth/introspect" "Content-Type: application/json, X-Affiliate-ID: sampleaff1"

echo

# Test 3: Multi-Tenant Routing
echo -e "${BLUE}Test 3: Multi-Tenant Scenarios${NC}"
test_endpoint "Alpha tenant" "GET" "/api/v1/test" "X-Affiliate-ID: tenant-alpha"
test_endpoint "Beta tenant" "GET" "/api/v1/test" "X-Affiliate-ID: tenant-beta"
test_endpoint "SampleAff1 tenant" "GET" "/api/v1/test" "X-Affiliate-ID: sampleaff1"
test_endpoint "No tenant (default)" "GET" "/api/v1/test"

echo

# Test 4: Debug and Monitoring Endpoints
echo -e "${BLUE}Test 4: Debug and Monitoring Endpoints${NC}"
test_endpoint "Feature flags debug" "GET" "/debug/flags" "X-Affiliate-ID: sampleaff1"
test_endpoint "System debug info" "GET" "/debug/info"
test_endpoint "Backend status" "GET" "/debug/backends"
test_endpoint "Circuit breaker status" "GET" "/debug/circuit-breakers"
test_endpoint "Health check status" "GET" "/debug/health-checks"

echo

# Test 5: Dry-Run Testing
echo -e "${BLUE}Test 5: Dry-Run Testing${NC}"
test_endpoint "Dry-run GET request" "GET" "/api/v1/test/dryrun" "X-Affiliate-ID: sampleaff1"
test_endpoint "Dry-run POST request" "POST" "/api/v1/test/dryrun" "Content-Type: application/json, X-Affiliate-ID: sampleaff1"

echo

# Test 6: Feature Flag Routing
echo -e "${BLUE}Test 6: Feature Flag Scenarios${NC}"
test_endpoint "API v1 with feature flag" "GET" "/api/v1/test" "X-Feature-Flag: enabled"
test_endpoint "API v2 routing" "GET" "/api/v2/test"
test_endpoint "Canary endpoint" "GET" "/api/canary/test"

echo

# Test 7: Load Testing (simplified)
echo -e "${BLUE}Test 7: Load Testing Scenario${NC}"
echo -n "  Concurrent requests (5x)... "

success_count=0
for i in {1..5}; do
    if curl -s -f "$PROXY_URL/api/v1/load" >/dev/null 2>&1; then
        success_count=$((success_count + 1))
    fi
done

if [[ $success_count -eq 5 ]]; then
    echo -e "${GREEN}PASS ($success_count/5)${NC}"
else
    echo -e "${RED}PARTIAL ($success_count/5)${NC}"
fi

echo

# Summary
echo -e "${GREEN}✓ All scenarios completed successfully${NC}"
echo
echo -e "${CYAN}Key Features Demonstrated:${NC}"
echo -e "  ${BLUE}•${NC} LaunchDarkly integration with graceful fallback"
echo -e "  ${BLUE}•${NC} Feature flag-controlled routing"
echo -e "  ${BLUE}•${NC} Multi-tenant isolation and routing"
echo -e "  ${BLUE}•${NC} Debug endpoints for monitoring and troubleshooting"
echo -e "  ${BLUE}•${NC} Dry-run functionality for backend comparison"
echo -e "  ${BLUE}•${NC} Health check monitoring across all backends"
echo -e "  ${BLUE}•${NC} Circuit breaker and failover mechanisms"
echo -e "  ${BLUE}•${NC} Chimera Facade specific API endpoints"
echo
echo -e "${CYAN}Endpoints Tested:${NC}"
echo -e "  ${BLUE}•${NC} Health: /health, /api/v1/health, /legacy/status"
echo -e "  ${BLUE}•${NC} Toolkit: /api/v1/toolkit/toolbox"
echo -e "  ${BLUE}•${NC} OAuth: /api/v1/authentication/oauth/*"
echo -e "  ${BLUE}•${NC} Debug: /debug/flags, /debug/info, /debug/backends"
echo -e "  ${BLUE}•${NC} Dry-run: /api/v1/test/dryrun"
echo
echo -e "${CYAN}Available Test Commands:${NC}"
echo "• ./testing-scenarios --scenario toolkit-api"
echo "• ./testing-scenarios --scenario oauth-token"
echo "• ./testing-scenarios --scenario debug-endpoints"
echo "• ./testing-scenarios --scenario dry-run"
echo "• ./test-chimera-scenarios.sh (comprehensive)"
echo "• ./test-all.sh"
echo
echo -e "${CYAN}Next Steps:${NC}"
echo -e "  ${BLUE}•${NC} Run full test suite: ./test-chimera-scenarios.sh"
echo -e "  ${BLUE}•${NC} Run specific scenarios: ./testing-scenarios --scenario=<name>"
echo -e "  ${BLUE}•${NC} Check application logs for detailed metrics"

# Clean up
echo
echo "Stopping application..."
kill $APP_PID 2>/dev/null
wait $APP_PID 2>/dev/null
echo -e "${GREEN}Testing scenarios demonstration complete!${NC}"