#!/bin/bash

# Health Check Testing Script
# Tests all health check scenarios for the reverse proxy

set -e

PROXY_URL="http://localhost:8080"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== Health Check Testing Scenarios ===${NC}"
echo

# Test direct backend health checks
echo "Testing direct backend health endpoints:"

backends=(
    "primary:9001:/health"
    "secondary:9002:/health" 
    "canary:9003:/health"
    "legacy:9004:/status"
    "monitoring:9005:/health"
    "unstable:9006:/health"
    "slow:9007:/health"
)

for backend_info in "${backends[@]}"; do
    IFS=':' read -r name port endpoint <<< "$backend_info"
    url="http://localhost:$port$endpoint"
    
    echo -n "  $name backend ($url)... "
    
    if curl -s -f "$url" >/dev/null 2>&1; then
        echo -e "${GREEN}HEALTHY${NC}"
    else
        echo -e "${RED}UNHEALTHY${NC}"
    fi
done

echo

# Test health checks through reverse proxy
echo "Testing health checks through reverse proxy:"

proxy_endpoints=(
    "/health:General health check"
    "/api/v1/health:API v1 health"
    "/api/v2/health:API v2 health"
    "/legacy/status:Legacy status"
    "/metrics/health:Monitoring health"
)

for endpoint_info in "${proxy_endpoints[@]}"; do
    IFS=':' read -r endpoint description <<< "$endpoint_info"
    url="$PROXY_URL$endpoint"
    
    echo -n "  $description ($endpoint)... "
    
    response=$(curl -s -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
done

echo

# Test health check with different tenants
echo "Testing health checks with tenant headers:"

tenants=("tenant-alpha" "tenant-beta" "tenant-canary")

for tenant in "${tenants[@]}"; do
    echo -n "  $tenant health check... "
    
    response=$(curl -s -w "%{http_code}" -H "X-Tenant-ID: $tenant" "$PROXY_URL/health" 2>/dev/null || echo "000")
    status_code="${response: -3}"
    
    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (HTTP $status_code)${NC}"
    fi
done

echo

# Test health check monitoring over time
echo "Testing health check stability (10 requests over 5 seconds):"
echo -n "  Stability test... "

success_count=0
for i in {1..10}; do
    if curl -s -f "$PROXY_URL/health" >/dev/null 2>&1; then
        success_count=$((success_count + 1))
    fi
    sleep 0.5
done

if [[ $success_count -ge 8 ]]; then
    echo -e "${GREEN}PASS ($success_count/10 successful)${NC}"
else
    echo -e "${RED}FAIL ($success_count/10 successful)${NC}"
fi

echo
echo -e "${GREEN}Health check testing completed${NC}"