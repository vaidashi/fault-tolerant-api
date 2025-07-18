#!/bin/bash

# This script tests the circuit breaker by generating errors

# URL to test (use the shipment creation endpoint as it might fail)
URL="http://localhost:8080/api/v1/orders"

# First, create a bunch of orders
echo "Creating 5 orders..."
ORDER_IDS=()
for i in $(seq 1 5); do
    RESPONSE=$(curl -s -X POST "$URL" \
        -H "Content-Type: application/json" \
        -d "{\"customer_id\":\"cust-circuit-$i\", \"amount\":99.99, \"description\":\"Circuit breaker test $i\"}")
    
    ORDER_ID=$(echo $RESPONSE | jq -r '.data.id')
    if [ "$ORDER_ID" != "null" ]; then
        ORDER_IDS+=("$ORDER_ID")
        echo "Created order $ORDER_ID"
    else
        echo "Failed to create order"
    fi
done

# Now try to create shipments for these orders rapidly to potentially trigger failures
echo "Creating shipments rapidly to trigger circuit breaker..."
for i in $(seq 1 20); do
    # Use random order ID from our list
    if [ ${#ORDER_IDS[@]} -gt 0 ]; then
        RANDOM_INDEX=$((RANDOM % ${#ORDER_IDS[@]}))
        ORDER_ID=${ORDER_IDS[$RANDOM_INDEX]}
        
        SHIPMENT_URL="http://localhost:8080/api/v1/orders/$ORDER_ID/shipments"
        STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHIPMENT_URL")
        echo "Request $i: $STATUS_CODE"
        
        # No delay to create pressure
    else
        echo "No orders available for testing"
        break
    fi
done

# Check circuit breaker status
echo "Circuit breaker status:"
curl -s http://localhost:8080/api/v1/admin/circuit-breaker | jq

# If circuit is open, try one more request to see the rejection
echo "Trying one more request..."
if [ ${#ORDER_IDS[@]} -gt 0 ]; then
    ORDER_ID=${ORDER_IDS[0]}
    SHIPMENT_URL="http://localhost:8080/api/v1/orders/$ORDER_ID/shipments"
    curl -i -X POST "$SHIPMENT_URL"
fi

echo "Test completed. Check the logs for circuit breaker behavior."