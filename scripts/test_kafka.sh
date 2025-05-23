#!/bin/bash

# Create and update orders to see Kafka messages flow through the system
echo "Creating new order to test Kafka integration..."
CREATE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-001", "amount":199.99, "description":"Testing Kafka integration"}')

ORDER_ID=$(echo $CREATE_RESPONSE | jq -r '.data.id')
echo "Created order with ID: $ORDER_ID"

echo "Waiting for Kafka processing..."
sleep 2

echo "Updating order status..."
curl -s -X PATCH http://localhost:8080/api/v1/orders/$ORDER_ID/status \
  -H "Content-Type: application/json" \
  -d '{"status":"approved"}'

echo "Waiting for Kafka processing..."
sleep 2

echo "Updating order details..."
curl -s -X PUT http://localhost:8080/api/v1/orders/$ORDER_ID \
  -H "Content-Type: application/json" \
  -d '{"amount":249.99, "description":"Updated Kafka test"}'

echo "Waiting for Kafka processing..."
sleep 2

echo "Getting final order state..."
curl -s -X GET http://localhost:8080/api/v1/orders/$ORDER_ID | jq

echo "Test completed. Check your logs for Kafka message processing."