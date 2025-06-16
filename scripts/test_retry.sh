#!/bin/bash

# This script tests the retry mechanisms by triggering failures in message processing

# First, create an order to generate a message
echo "Creating an order to generate a message..."
CREATE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-retry", "amount":299.99, "description":"Testing retry mechanism"}')

ORDER_ID=$(echo $CREATE_RESPONSE | jq -r '.data.id')
echo "Created order with ID: $ORDER_ID"

# Wait to allow the outbox processor to process the message
echo "Waiting for message processing..."
sleep 5

# Check the DLQ (assuming some messages might have failed due to Kafka connection issues)
echo "Checking Dead Letter Queue..."
curl -s -X GET "http://localhost:8080/api/v1/admin/dead-letters" | jq

# If there are messages in the DLQ, try to retry the first one
echo "Attempting to retry a message (if any)..."
DLQ_RESPONSE=$(curl -s -X GET "http://localhost:8080/api/v1/admin/dead-letters" | jq)
DLQ_COUNT=$(echo $DLQ_RESPONSE | jq '.data.items | length')

if [ "$DLQ_COUNT" -gt 0 ]; then
  DLQ_ID=$(echo $DLQ_RESPONSE | jq -r '.data.items[0].id')
  echo "Retrying message with ID: $DLQ_ID"
  curl -s -X POST "http://localhost:8080/api/v1/admin/dead-letters/$DLQ_ID/retry" | jq
else
  echo "No messages in the Dead Letter Queue"
fi

echo "Test completed. Check your logs for retry behavior."