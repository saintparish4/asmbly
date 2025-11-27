#!/bin/bash

# TraceFlow Manual Testing Script
# Quick manual verification of collector endpoints

set -e

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:9090}"

echo "=== TraceFlow Manual Test ==="
echo "Collector URL: $COLLECTOR_URL"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test 1: Health check
echo "Test 1: Health Check"
echo "GET $COLLECTOR_URL/health"
RESPONSE=$(curl -s "$COLLECTOR_URL/health")
if echo "$RESPONSE" | grep -q "healthy"; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Test 2: Metrics endpoint
echo "Test 2: Metrics"
echo "GET $COLLECTOR_URL/metrics"
RESPONSE=$(curl -s "$COLLECTOR_URL/metrics")
if echo "$RESPONSE" | grep -q "traceflow_spans"; then
    echo -e "${GREEN}✓ Metrics endpoint working${NC}"
    echo "$RESPONSE"
else
    echo -e "${RED}✗ Metrics endpoint failed${NC}"
    exit 1
fi
echo ""

# Generate test IDs
TRACE_ID=$(openssl rand -hex 16)
SPAN_ID_1=$(openssl rand -hex 8)
SPAN_ID_2=$(openssl rand -hex 8)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Test 3: Submit single span
echo "Test 3: Submit Single Span"
echo "POST $COLLECTOR_URL/api/v1/spans"

SPAN_JSON=$(cat <<EOF
{
  "trace_id": "$TRACE_ID",
  "span_id": "$SPAN_ID_1",
  "service_name": "frontend",
  "operation_name": "page-load",
  "start_time": "$TIMESTAMP",
  "duration": 100000000,
  "status": "ok",
  "span_kind": "client",
  "tags": {
    "http.method": "GET",
    "http.url": "/",
    "http.status_code": "200"
  }
}
EOF
)

echo "Payload:"
echo "$SPAN_JSON" | jq '.'

RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d "$SPAN_JSON" \
    "$COLLECTOR_URL/api/v1/spans")

if echo "$RESPONSE" | grep -q "accepted"; then
    echo -e "${GREEN}✓ Span accepted${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Span submission failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Wait for async processing
echo "Waiting for span to be processed..."
sleep 1
echo ""

# Test 4: Retrieve trace
echo "Test 4: Retrieve Trace"
echo "GET $COLLECTOR_URL/api/v1/traces/$TRACE_ID"

RESPONSE=$(curl -s "$COLLECTOR_URL/api/v1/traces/$TRACE_ID")
if echo "$RESPONSE" | grep -q "$TRACE_ID"; then
    echo -e "${GREEN}✓ Trace retrieved successfully${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Trace retrieval failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Test 5: Submit batch of spans
echo "Test 5: Submit Batch of Spans"
echo "POST $COLLECTOR_URL/api/v1/spans/batch"

BATCH_JSON=$(cat <<EOF
[
  {
    "trace_id": "$TRACE_ID",
    "span_id": "$SPAN_ID_2",
    "service_name": "api",
    "operation_name": "get-users",
    "start_time": "$TIMESTAMP",
    "duration": 50000000,
    "status": "ok",
    "span_kind": "server",
    "tags": {
      "db.query": "SELECT * FROM users"
    }
  }
]
EOF
)

echo "Payload:"
echo "$BATCH_JSON" | jq '.'

RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d "$BATCH_JSON" \
    "$COLLECTOR_URL/api/v1/spans/batch")

if echo "$RESPONSE" | grep -q "accepted"; then
    echo -e "${GREEN}✓ Batch accepted${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Batch submission failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Wait for processing
sleep 1

# Test 6: Query traces with filters
echo "Test 6: Query Traces with Filters"
echo "GET $COLLECTOR_URL/api/v1/traces?service=frontend&limit=10"

RESPONSE=$(curl -s "$COLLECTOR_URL/api/v1/traces?service=frontend&limit=10")
if echo "$RESPONSE" | grep -q "traces"; then
    echo -e "${GREEN}✓ Query successful${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Query failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Test 7: Get services list
echo "Test 7: Get Services List"
echo "GET $COLLECTOR_URL/api/v1/services"

RESPONSE=$(curl -s "$COLLECTOR_URL/api/v1/services")
if echo "$RESPONSE" | grep -q "services"; then
    echo -e "${GREEN}✓ Services list retrieved${NC}"
    echo "$RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Services list failed${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo ""

# Final metrics check
echo "=== Final Metrics ==="
curl -s "$COLLECTOR_URL/metrics" | grep traceflow_spans
echo ""

echo -e "${GREEN}=== All Tests Passed! ===${NC}"
echo ""
echo "Trace ID for manual inspection: $TRACE_ID"
echo "View trace: $COLLECTOR_URL/api/v1/traces/$TRACE_ID"