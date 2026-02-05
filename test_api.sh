#!/bin/bash

echo "Testing User Activity Tracker API"
echo "================================="
echo ""

# Base URL
BASE_URL="http://localhost:8080/api"

# Test 1: Register a new client
echo "1. Testing client registration..."
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/register" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Client",
    "email": "test@example.com"
  }')

echo "Response: $REGISTER_RESPONSE"
CLIENT_ID=$(echo $REGISTER_RESPONSE | grep -o '"client_id":"[^"]*"' | cut -d'"' -f4)
API_KEY=$(echo $REGISTER_RESPONSE | grep -o '"api_key":"[^"]*"' | cut -d'"' -f4)
TOKEN=$(echo $REGISTER_RESPONSE | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

echo "Client ID: $CLIENT_ID"
echo ""

# Test 2: Record API hits
echo "2. Testing API hit logging..."
for i in {1..5}; do
  RESPONSE=$(curl -s -X POST "$BASE_URL/logs" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "{\"endpoint\": \"/api/test/$i\"}")
  echo "Hit $i: $RESPONSE"
  sleep 0.5
done
echo ""

# Test 3: Get daily usage
echo "3. Testing daily usage endpoint..."
DAILY_USAGE=$(curl -s -X GET "$BASE_URL/usage/daily" \
  -H "X-API-Key: $API_KEY")
echo "Daily Usage: $DAILY_USAGE"
echo ""

# Test 4: Get top clients
echo "4. Testing top clients endpoint..."
TOP_CLIENTS=$(curl -s -X GET "$BASE_URL/usage/top" \
  -H "X-API-Key: $API_KEY")
echo "Top Clients: $TOP_CLIENTS"
echo ""

# Test 5: Health check
echo "5. Testing health check..."
HEALTH=$(curl -s -X GET "http://localhost:8080/health")
echo "Health: $HEALTH"
echo ""

# Test 6: Swagger documentation
echo "6. Swagger documentation available at: http://localhost:8080/swagger/index.html"
echo ""

echo "All tests completed!"