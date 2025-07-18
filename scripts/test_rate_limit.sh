#!/bin/bash

# This script tests the rate limiting functionality by sending many requests in quick succession

# URL to test
URL="http://localhost:8080/api/v1/orders"

# Number of concurrent requests
CONCURRENCY=10

# Total number of requests
TOTAL_REQUESTS=50

# Function to make a single request and capture response code
make_request() {
    STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$URL")
    echo "$STATUS_CODE"
}

# Run concurrent requests
echo "Sending $TOTAL_REQUESTS requests with concurrency $CONCURRENCY..."
for i in $(seq 1 $TOTAL_REQUESTS); do
    # Check if we've reached concurrency limit
    if [ "$(jobs -r | wc -l)" -ge "$CONCURRENCY" ]; then
        # Wait for a job to finish before starting a new one
        wait -n
    fi
    
    # Start a new request in the background
    make_request &
    
    # Show progress
    if [ $((i % 10)) -eq 0 ]; then
        echo "Sent $i requests..."
    fi
done

# Wait for all background jobs to finish
wait

echo "All requests completed. Check the logs for rate limiting behavior."

# Get rate limiter metrics
echo "Current rate limiter metrics:"
curl -s http://localhost:8080/api/v1/admin/rate-limits | jq

# Get circuit breaker status
echo "Current circuit breaker status:"
curl -s http://localhost:8080/api/v1/admin/circuit-breaker | jq