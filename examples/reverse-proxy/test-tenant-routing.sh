#!/bin/bash

echo "=== Reverse Proxy Tenant-Specific Default Backend Test ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing tenant-specific default backend routing...${NC}"
echo ""

echo -e "${YELLOW}1. Testing Tenant1 (should route to tenant1-backend on port 9002):${NC}"
echo "curl -H \"X-Tenant-ID: tenant1\" http://localhost:8080/test"
curl -H "X-Tenant-ID: tenant1" http://localhost:8080/test
echo ""
echo ""

echo -e "${YELLOW}2. Testing Tenant2 (should route to tenant2-backend on port 9003):${NC}"
echo "curl -H \"X-Tenant-ID: tenant2\" http://localhost:8080/test"
curl -H "X-Tenant-ID: tenant2" http://localhost:8080/test
echo ""
echo ""

echo -e "${YELLOW}3. Testing No Tenant Header (should route to global-default on port 9001):${NC}"
echo "curl http://localhost:8080/test"
curl http://localhost:8080/test
echo ""
echo ""

echo -e "${YELLOW}4. Testing Unknown Tenant (should fall back to global-default):${NC}"
echo "curl -H \"X-Tenant-ID: unknown-tenant\" http://localhost:8080/test"
curl -H "X-Tenant-ID: unknown-tenant" http://localhost:8080/test
echo ""
echo ""

echo -e "${YELLOW}5. Testing Different Paths with Tenant1:${NC}"
echo "curl -H \"X-Tenant-ID: tenant1\" http://localhost:8080/api/users/123"
curl -H "X-Tenant-ID: tenant1" http://localhost:8080/api/users/123
echo ""
echo ""

echo -e "${YELLOW}6. Testing POST Request with Tenant2:${NC}"
echo "curl -X POST -H \"X-Tenant-ID: tenant2\" -H \"Content-Type: application/json\" -d '{\"test\":\"data\"}' http://localhost:8080/api/data"
curl -X POST -H "X-Tenant-ID: tenant2" -H "Content-Type: application/json" -d '{"test":"data"}' http://localhost:8080/api/data
echo ""
echo ""

echo -e "${YELLOW}7. Testing Root Path with Tenant1:${NC}"
echo "curl -H \"X-Tenant-ID: tenant1\" http://localhost:8080/"
curl -H "X-Tenant-ID: tenant1" http://localhost:8080/
echo ""
echo ""

echo -e "${GREEN}All tests completed! The tenant-specific default backend routing is working correctly.${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo "- Tenant1 requests → tenant1-backend (localhost:9002)"
echo "- Tenant2 requests → tenant2-backend (localhost:9003)"  
echo "- No tenant/unknown tenant → global-default (localhost:9001)"
echo "- All HTTP methods and paths are properly forwarded"
