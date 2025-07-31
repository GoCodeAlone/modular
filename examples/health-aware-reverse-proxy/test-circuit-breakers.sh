#!/bin/bash

# Script to test circuit breaker and health status integration

echo "Testing Circuit Breaker and Health Status Integration"
echo "===================================================="

echo
echo "1. Initial health status:"
curl -s http://localhost:8080/health | jq .

echo
echo "2. Testing unreachable API (should trigger circuit breaker):"
for i in {1..3}; do
    echo "  Request $i:"
    response=$(curl -w "HTTP_CODE:%{http_code}" -s http://localhost:8080/api/unreachable)
    echo "    Response: $response"
done

echo
echo "3. Health status after circuit breaker triggers:"
curl -s http://localhost:8080/health | jq .

echo
echo "4. Detailed circuit breaker status for unreachable-api:"
curl -s http://localhost:8080/metrics/reverseproxy/health | jq '.backend_details."unreachable-api" | {backend_id, healthy, circuit_breaker_open, circuit_breaker_state, circuit_failure_count}'

echo
echo "5. Testing intermittent API (trigger failures):"
for i in {1..6}; do
    echo "  Request $i:"
    response=$(curl -w "HTTP_CODE:%{http_code}" -s http://localhost:8080/api/intermittent)
    echo "    Response: $response"
done

echo
echo "6. Health status after intermittent API failures:"
curl -s http://localhost:8080/health | jq .

echo
echo "7. Detailed circuit breaker status for intermittent-api:"
curl -s http://localhost:8080/metrics/reverseproxy/health | jq '.backend_details."intermittent-api" | {backend_id, healthy, circuit_breaker_open, circuit_breaker_state, circuit_failure_count}'

echo
echo "Test completed."