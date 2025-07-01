#!/bin/bash

# This script tests the integration with the simulated warehouse service

# First, create an order
echo "Creating an order..."
CREATE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-warehouse", "amount":399.99, "description":"Testing warehouse integration"}')

ORDER_ID=$(echo $CREATE_RESPONSE | jq -r '.data.id')
echo "Created order with ID: $ORDER_ID"

# Update the order status to approved (required before shipping)
echo "Approving order..."
curl -s -X PATCH http://localhost:8080/api/v1/orders/$ORDER_ID/status \
  -H "Content-Type: application/json" \
  -d '{"status":"approved"}'

# Create a shipment for the order
echo "Creating shipment..."
SHIPMENT_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders/$ORDER_ID/shipments)
echo $SHIPMENT_RESPONSE | jq

SHIPMENT_ID=$(echo $SHIPMENT_RESPONSE | jq -r '.data.id')

# Get shipments for the order
echo "Getting shipments for order..."
curl -s -X GET http://localhost:8080/api/v1/orders/$ORDER_ID/shipments | jq

# Sync the shipment status with the warehouse (may succeed or fail randomly)
echo "Syncing shipment status..."
curl -s -X POST http://localhost:8080/api/v1/shipments/$SHIPMENT_ID/sync | jq

# Try syncing again to see retry mechanism in action
echo "Syncing shipment status again..."
curl -s -X POST http://localhost:8080/api/v1/shipments/$SHIPMENT_ID/sync | jq

echo "Test completed. Check your logs for retry behavior."