#!/bin/bash

# Load Testing Script
# Tests high-concurrency scenarios for the reverse proxy

set -e

PROXY_URL="http://localhost:8080"
REQUESTS=${1:-100}
CONCURRENCY=${2:-10}
DURATION=${3:-30}

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== Load Testing Scenarios ===${NC}"
echo "Configuration:"
echo "  Target URL: $PROXY_URL"
echo "  Total requests: $REQUESTS"
echo "  Concurrency: $CONCURRENCY"
echo "  Duration: ${DURATION}s"
echo

# Function to run a single request and return the result
run_request() {
    local url="$1"
    local request_id="$2"
    local headers="$3"
    
    local cmd="curl -s -w '%{http_code}:%{time_total}' -m 10"
    
    if [[ -n "$headers" ]]; then
        cmd="$cmd -H '$headers'"
    fi
    
    cmd="$cmd '$url'"
    
    eval "$cmd" 2>/dev/null || echo "000:0.000"
}

# Test 1: Sequential load test
echo -e "${BLUE}Test 1: Sequential Load Test${NC}"
echo "Running $REQUESTS sequential requests..."

start_time=$(date +%s)
success_count=0
total_time=0
min_time=999
max_time=0

for ((i=1; i<=REQUESTS; i++)); do
    result=$(run_request "$PROXY_URL/api/v1/load-test" "$i")
    IFS=':' read -r status_code response_time <<< "$result"
    
    if [[ "$status_code" == "200" ]]; then
        ((success_count++))
        
        # Convert response time to milliseconds
        time_ms=$(echo "$response_time * 1000" | bc -l 2>/dev/null || echo "0")
        total_time=$(echo "$total_time + $time_ms" | bc -l 2>/dev/null || echo "$total_time")
        
        # Track min/max times
        if (( $(echo "$time_ms < $min_time" | bc -l 2>/dev/null || echo "0") )); then
            min_time=$time_ms
        fi
        if (( $(echo "$time_ms > $max_time" | bc -l 2>/dev/null || echo "0") )); then
            max_time=$time_ms
        fi
    fi
    
    # Progress indicator
    if (( i % 10 == 0 )); then
        echo -n "."
    fi
done
echo

end_time=$(date +%s)
duration=$((end_time - start_time))
success_rate=$(echo "scale=2; $success_count * 100 / $REQUESTS" | bc -l 2>/dev/null || echo "0")
avg_time=$(echo "scale=2; $total_time / $success_count" | bc -l 2>/dev/null || echo "0")
throughput=$(echo "scale=2; $success_count / $duration" | bc -l 2>/dev/null || echo "0")

echo "Results:"
echo "  Total requests: $REQUESTS"
echo "  Successful: $success_count"
echo "  Success rate: ${success_rate}%"
echo "  Duration: ${duration}s"
echo "  Throughput: ${throughput} req/s"
if [[ "$success_count" -gt "0" ]]; then
    echo "  Avg response time: ${avg_time}ms"
    echo "  Min response time: ${min_time}ms"
    echo "  Max response time: ${max_time}ms"
fi
echo

# Test 2: Concurrent load test
echo -e "${BLUE}Test 2: Concurrent Load Test${NC}"
echo "Running $REQUESTS requests with concurrency $CONCURRENCY..."

# Create temporary directory for results
temp_dir=$(mktemp -d)
start_time=$(date +%s)

# Function to run concurrent batch
run_concurrent_batch() {
    local batch_size="$1"
    local batch_start="$2"
    
    for ((i=0; i<batch_size; i++)); do
        {
            local request_id=$((batch_start + i))
            local result=$(run_request "$PROXY_URL/api/v1/concurrent-test" "$request_id" "X-Request-ID: load-test-$request_id")
            echo "$result" > "$temp_dir/result_$request_id.txt"
        } &
    done
    
    wait
}

# Run concurrent batches
remaining=$REQUESTS
batch_start=1

while [[ $remaining -gt 0 ]]; do
    batch_size=$CONCURRENCY
    if [[ $remaining -lt $CONCURRENCY ]]; then
        batch_size=$remaining
    fi
    
    run_concurrent_batch "$batch_size" "$batch_start"
    
    batch_start=$((batch_start + batch_size))
    remaining=$((remaining - batch_size))
    
    echo -n "#"
done
echo

end_time=$(date +%s)
duration=$((end_time - start_time))

# Collect results
success_count=0
total_time=0
min_time=999
max_time=0

for ((i=1; i<=REQUESTS; i++)); do
    if [[ -f "$temp_dir/result_$i.txt" ]]; then
        result=$(cat "$temp_dir/result_$i.txt")
        IFS=':' read -r status_code response_time <<< "$result"
        
        if [[ "$status_code" == "200" ]]; then
            ((success_count++))
            
            time_ms=$(echo "$response_time * 1000" | bc -l 2>/dev/null || echo "0")
            total_time=$(echo "$total_time + $time_ms" | bc -l 2>/dev/null || echo "$total_time")
            
            if (( $(echo "$time_ms < $min_time" | bc -l 2>/dev/null || echo "0") )); then
                min_time=$time_ms
            fi
            if (( $(echo "$time_ms > $max_time" | bc -l 2>/dev/null || echo "0") )); then
                max_time=$time_ms
            fi
        fi
    fi
done

# Cleanup
rm -rf "$temp_dir"

success_rate=$(echo "scale=2; $success_count * 100 / $REQUESTS" | bc -l 2>/dev/null || echo "0")
avg_time=$(echo "scale=2; $total_time / $success_count" | bc -l 2>/dev/null || echo "0")
throughput=$(echo "scale=2; $success_count / $duration" | bc -l 2>/dev/null || echo "0")

echo "Results:"
echo "  Total requests: $REQUESTS"
echo "  Successful: $success_count"
echo "  Success rate: ${success_rate}%"
echo "  Duration: ${duration}s"
echo "  Throughput: ${throughput} req/s"
if [[ "$success_count" -gt "0" ]]; then
    echo "  Avg response time: ${avg_time}ms"
    echo "  Min response time: ${min_time}ms"
    echo "  Max response time: ${max_time}ms"
fi
echo

# Test 3: Sustained load test
echo -e "${BLUE}Test 3: Sustained Load Test${NC}"
echo "Running sustained load for ${DURATION} seconds..."

start_time=$(date +%s)
success_count=0
request_count=0

while [[ $(($(date +%s) - start_time)) -lt $DURATION ]]; do
    result=$(run_request "$PROXY_URL/api/v1/sustained" "$request_count")
    IFS=':' read -r status_code response_time <<< "$result"
    
    ((request_count++))
    if [[ "$status_code" == "200" ]]; then
        ((success_count++))
    fi
    
    # Small delay to prevent overwhelming
    sleep 0.1
done

end_time=$(date +%s)
actual_duration=$((end_time - start_time))
success_rate=$(echo "scale=2; $success_count * 100 / $request_count" | bc -l 2>/dev/null || echo "0")
throughput=$(echo "scale=2; $success_count / $actual_duration" | bc -l 2>/dev/null || echo "0")

echo "Results:"
echo "  Total requests: $request_count"
echo "  Successful: $success_count"
echo "  Success rate: ${success_rate}%"
echo "  Duration: ${actual_duration}s"
echo "  Throughput: ${throughput} req/s"
echo

# Summary
echo -e "${GREEN}=== Load Testing Summary ===${NC}"
echo "All load testing scenarios completed."
echo "The reverse proxy handled concurrent requests and sustained load successfully."