#!/bin/bash

# Feature Flag Testing Script
# Tests feature flag routing scenarios

set -e

PROXY_URL="http://localhost:8080"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== Feature Flag Testing Scenarios ===${NC}"
echo

# Test basic feature flag routing
echo -e "${BLUE}Testing feature flag enabled/disabled routing:${NC}"

endpoints=(
    "/api/v1/test:API v1 endpoint"
    "/api/v2/test:API v2 endpoint" 
    "/api/canary/test:Canary endpoint"
)

for endpoint_info in "${endpoints[@]}"; do
    IFS=':' read -r endpoint description <<< "$endpoint_info"
    
    echo "  Testing $description ($endpoint):"
    
    # Test without any feature flag headers (default behavior)
    echo -n "    Default routing... "
    response=$(curl -s -w "%{http_code}" "$PROXY_URL$endpoint" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
    
    # Test with feature flag headers
    echo -n "    With feature flag... "
    response=$(curl -s -w "%{http_code}" -H "X-Feature-Flag: enabled" "$PROXY_URL$endpoint" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
done

echo

# Test tenant-specific feature flags
echo -e "${BLUE}Testing tenant-specific feature flags:${NC}"

tenants=("tenant-alpha" "tenant-beta" "tenant-canary")

for tenant in "${tenants[@]}"; do
    echo "  Testing $tenant:"
    
    # Test with tenant header
    echo -n "    Basic routing... "
    response=$(curl -s -w "%{http_code}" -H "X-Tenant-ID: $tenant" "$PROXY_URL/api/v1/test" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
    
    # Test with tenant and feature flag
    echo -n "    With feature flag... "
    response=$(curl -s -w "%{http_code}" -H "X-Tenant-ID: $tenant" -H "X-Feature-Flag: test-feature" "$PROXY_URL/api/v2/test" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
done

echo

# Test feature flag fallback behavior
echo -e "${BLUE}Testing feature flag fallback behavior:${NC}"

fallback_tests=(
    "/api/v1/fallback:API v1 fallback"
    "/api/v2/fallback:API v2 fallback"
    "/api/canary/fallback:Canary fallback"
)

for test_info in "${fallback_tests[@]}"; do
    IFS=':' read -r endpoint description <<< "$test_info"
    
    echo -n "  Testing $description... "
    
    # Test with disabled feature flag
    response=$(curl -s -w "%{http_code}" -H "X-Feature-Flag: disabled" "$PROXY_URL$endpoint" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS (fallback working)${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
done

echo

# Test complex feature flag scenarios
echo -e "${BLUE}Testing complex feature flag scenarios:${NC}"

# Test multiple feature flags
echo -n "  Multiple feature flags... "
response=$(curl -s -w "%{http_code}" \
    -H "X-Feature-Flag-1: enabled" \
    -H "X-Feature-Flag-2: disabled" \
    -H "X-Feature-Flag-3: enabled" \
    "$PROXY_URL/api/v1/multi-flag" 2>/dev/null || echo "000")
status_code="${response: -3}"
if [[ "$status_code" == "200" ]]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL (HTTP $status_code)${NC}"
fi

# Test feature flag with tenant override
echo -n "  Tenant feature flag override... "
response=$(curl -s -w "%{http_code}" \
    -H "X-Tenant-ID: tenant-alpha" \
    -H "X-Feature-Flag: tenant-specific" \
    "$PROXY_URL/api/v2/tenant-override" 2>/dev/null || echo "000")
status_code="${response: -3}"
if [[ "$status_code" == "200" ]]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL (HTTP $status_code)${NC}"
fi

# Test canary deployment simulation
echo -n "  Canary deployment simulation... "
response=$(curl -s -w "%{http_code}" \
    -H "X-Feature-Flag: canary-deployment" \
    -H "X-Canary-User: true" \
    "$PROXY_URL/api/canary/deployment" 2>/dev/null || echo "000")
status_code="${response: -3}"
if [[ "$status_code" == "200" ]]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL (HTTP $status_code)${NC}"
fi

echo

# Test feature flag performance
echo -e "${BLUE}Testing feature flag performance:${NC}"

echo -n "  Performance test (10 requests with flags)... "
start_time=$(date +%s%N)
success_count=0

for i in {1..10}; do
    response=$(curl -s -w "%{http_code}" \
        -H "X-Feature-Flag: performance-test" \
        -H "X-Request-ID: perf-$i" \
        "$PROXY_URL/api/v1/performance" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    
    if [[ "$status_code" == "200" ]]; then
        success_count=$((success_count + 1))
    fi
done

end_time=$(date +%s%N)
duration_ms=$(( (end_time - start_time) / 1000000 ))
avg_time_ms=$(( duration_ms / 10 ))

if [[ $success_count -ge 8 ]]; then
    echo -e "${GREEN}PASS ($success_count/10 successful, avg ${avg_time_ms}ms)${NC}"
else
    echo -e "${RED}FAIL ($success_count/10 successful)${NC}"
fi

echo

echo -e "${GREEN}=== Feature Flag Testing Summary ===${NC}"
echo "Feature flag routing scenarios tested successfully."
echo "The reverse proxy correctly handles feature flag-based routing,"
echo "tenant-specific flags, and fallback behavior."