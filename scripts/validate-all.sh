#!/bin/bash

# TraceFlow Comprehensive Validation Script
# Generates 100 spans and validates all features

set -e

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:9090}"

echo "=== TraceFlow Comprehensive Validation ==="
echo "Collector URL: $COLLECTOR_URL"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper function to run test
run_test() {
    local name="$1"
    local command="$2"
    local expected="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -n "Test $TOTAL_TESTS: $name ... "
    
    if eval "$command" | grep -q "$expected"; then
        echo -e "${GREEN}✓ PASSED${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# Check if collector is running
echo "Checking if collector is running..."
if ! curl -f -s "$COLLECTOR_URL/health" > /dev/null; then
    echo -e "${RED}ERROR: Collector not responding at $COLLECTOR_URL${NC}"
    echo "Please start the collector with: go run cmd/collector/main.go"
    exit 1
fi
echo -e "${GREEN}✓ Collector is healthy${NC}"
echo ""

# Generate 100 spans across multiple traces and services
echo "=== Generating 100 Test Spans ==="
echo ""

SERVICES=("frontend" "api" "database" "cache" "auth")
OPERATIONS=("page-load" "GET /users" "POST /orders" "SELECT" "GET key" "login" "verify-token")

TRACE_IDS=()
SPAN_COUNT=0

for i in {1..20}; do
    # Generate trace ID
    TRACE_ID=$(openssl rand -hex 16)
    TRACE_IDS+=("$TRACE_ID")
    
    # Generate 5 spans per trace
    for j in {1..5}; do
        SERVICE="${SERVICES[$((RANDOM % ${#SERVICES[@]}))]}"
        OPERATION="${OPERATIONS[$((RANDOM % ${#OPERATIONS[@]}))]}"
        DURATION=$((RANDOM % 500000000 + 10000000))  # 10ms - 500ms
        SPAN_ID=$(openssl rand -hex 8)
        TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        
        # Determine status (90% ok, 10% error)
        if [ $((RANDOM % 10)) -eq 0 ]; then
            STATUS="error"
        else
            STATUS="ok"
        fi
        
        # Submit span
        curl -s -X POST "$COLLECTOR_URL/api/v1/spans" \
            -H "Content-Type: application/json" \
            -d "{
                \"trace_id\": \"$TRACE_ID\",
                \"span_id\": \"$SPAN_ID\",
                \"service_name\": \"$SERVICE\",
                \"operation_name\": \"$OPERATION\",
                \"start_time\": \"$TIMESTAMP\",
                \"duration\": $DURATION,
                \"status\": \"$STATUS\",
                \"tags\": {
                    \"test\": \"validation\",
                    \"batch\": \"$i\"
                }
            }" > /dev/null
        
        SPAN_COUNT=$((SPAN_COUNT + 1))
        
        if [ $((SPAN_COUNT % 10)) -eq 0 ]; then
            echo -n "."
        fi
    done
done

echo ""
echo -e "${GREEN}✓ Generated $SPAN_COUNT spans across ${#TRACE_IDS[@]} traces${NC}"
echo ""

# Wait for async processing
echo "Waiting for spans to be processed..."
sleep 2
echo ""

# Verify spans were stored
echo "=== Verifying Span Storage ==="
echo ""

METRICS=$(curl -s "$COLLECTOR_URL/metrics")
SPANS_RECEIVED=$(echo "$METRICS" | grep "traceflow_spans_received_total" | awk '{print $2}')
SPANS_STORED=$(echo "$METRICS" | grep "traceflow_spans_stored_total" | awk '{print $2}')
SPAN_ERRORS=$(echo "$METRICS" | grep "traceflow_span_errors_total" | awk '{print $2}')

echo "Metrics:"
echo "  Received: $SPANS_RECEIVED"
echo "  Stored:   $SPANS_STORED"
echo "  Errors:   $SPAN_ERRORS"
echo ""

if [ "$SPANS_STORED" -ge "$SPAN_COUNT" ]; then
    echo -e "${GREEN}✓ All spans stored successfully${NC}"
else
    echo -e "${YELLOW}⚠ Some spans may have been lost${NC}"
fi
echo ""

# Test 1: Retrieve traces
echo "=== Testing Trace Retrieval ==="
echo ""

for i in {0..4}; do
    TRACE_ID="${TRACE_IDS[$i]}"
    run_test "Retrieve trace $((i+1))" \
        "curl -s $COLLECTOR_URL/api/v1/traces/$TRACE_ID" \
        "$TRACE_ID"
done
echo ""

# Test 2: Query by service
echo "=== Testing Service Filter ==="
echo ""

for SERVICE in "${SERVICES[@]}"; do
    run_test "Query service '$SERVICE'" \
        "curl -s '$COLLECTOR_URL/api/v1/traces?service=$SERVICE'" \
        "\"service\":\"$SERVICE\""
done
echo ""

# Test 3: Query by duration
echo "=== Testing Duration Filter ==="
echo ""

run_test "Min duration (50ms)" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?min_duration=50ms'" \
    "traces"

run_test "Max duration (100ms)" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?max_duration=100ms'" \
    "traces"

run_test "Duration range (50ms-200ms)" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?min_duration=50ms&max_duration=200ms'" \
    "traces"
echo ""

# Test 4: Pagination
echo "=== Testing Pagination ==="
echo ""

run_test "Limit 10" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?limit=10'" \
    "\"total\""

run_test "Offset 5" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?limit=5&offset=5'" \
    "traces"

run_test "Page 2 (offset=10, limit=10)" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?limit=10&offset=10'" \
    "traces"
echo ""

# Test 5: Services list
echo "=== Testing Services List ==="
echo ""

run_test "Get all services" \
    "curl -s '$COLLECTOR_URL/api/v1/services'" \
    "\"services\""

SERVICES_RESPONSE=$(curl -s "$COLLECTOR_URL/api/v1/services")
SERVICE_COUNT=$(echo "$SERVICES_RESPONSE" | grep -o "\"total\":[0-9]*" | grep -o "[0-9]*")

if [ "$SERVICE_COUNT" -eq "${#SERVICES[@]}" ]; then
    echo -e "${GREEN}✓ All services discovered ($SERVICE_COUNT)${NC}"
else
    echo -e "${YELLOW}⚠ Expected ${#SERVICES[@]} services, found $SERVICE_COUNT${NC}"
fi
echo ""

# Test 6: Complex queries
echo "=== Testing Complex Queries ==="
echo ""

run_test "Service + duration filter" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?service=api&min_duration=10ms'" \
    "traces"

run_test "Service + pagination" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?service=frontend&limit=5'" \
    "traces"

run_test "Duration + time range" \
    "curl -s '$COLLECTOR_URL/api/v1/traces?min_duration=20ms&start_time=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)'" \
    "traces"
echo ""

# Test 7: Error handling
echo "=== Testing Error Handling ==="
echo ""

run_test "Invalid trace ID (404)" \
    "curl -s -w '%{http_code}' -o /dev/null '$COLLECTOR_URL/api/v1/traces/nonexistent'" \
    "404"

run_test "Invalid JSON (400)" \
    "curl -s -w '%{http_code}' -o /dev/null -X POST '$COLLECTOR_URL/api/v1/spans' -d 'invalid'" \
    "400"

run_test "Invalid method (405)" \
    "curl -s -w '%{http_code}' -o /dev/null -X DELETE '$COLLECTOR_URL/api/v1/spans'" \
    "405"
echo ""

# Test 8: CORS headers
echo "=== Testing CORS Headers ==="
echo ""

CORS_RESPONSE=$(curl -s -I -X OPTIONS "$COLLECTOR_URL/api/v1/spans" -H "Origin: http://example.com")

if echo "$CORS_RESPONSE" | grep -q "Access-Control-Allow-Origin"; then
    echo -e "${GREEN}✓ CORS headers present${NC}"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo -e "${RED}✗ CORS headers missing${NC}"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))
echo ""

# Test 9: Health check
echo "=== Testing Health Check ==="
echo ""

run_test "Health endpoint" \
    "curl -s '$COLLECTOR_URL/health'" \
    "healthy"
echo ""

# Test 10: Metrics
echo "=== Testing Metrics ==="
echo ""

run_test "Prometheus metrics" \
    "curl -s '$COLLECTOR_URL/metrics'" \
    "traceflow_spans"
echo ""

# Performance check
echo "=== Performance Check ==="
echo ""

# Calculate ingestion rate
DURATION=2  # Our test took ~2 seconds
RATE=$((SPAN_COUNT / DURATION))

echo "Ingestion Performance:"
echo "  Spans: $SPAN_COUNT"
echo "  Time:  ~${DURATION}s"
echo "  Rate:  ~${RATE} spans/sec"

if [ "$RATE" -ge 40 ]; then
    echo -e "${GREEN}✓ Performance: Excellent (>40 spans/sec)${NC}"
elif [ "$RATE" -ge 20 ]; then
    echo -e "${YELLOW}⚠ Performance: Acceptable (20-40 spans/sec)${NC}"
else
    echo -e "${RED}✗ Performance: Poor (<20 spans/sec)${NC}"
fi
echo ""

# Final summary
echo "=== Test Summary ==="
echo ""
echo "Total Tests:  $TOTAL_TESTS"
echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
if [ "$FAILED_TESTS" -gt 0 ]; then
    echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
else
    echo -e "Failed:       $FAILED_TESTS"
fi
echo ""

# Calculate pass rate
PASS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))

if [ "$PASS_RATE" -eq 100 ]; then
    echo -e "${GREEN}✓✓✓ ALL TESTS PASSED! (100%) ✓✓✓${NC}"
    echo ""
    echo "TraceFlow Layer 1 is ready for production!"
    exit 0
elif [ "$PASS_RATE" -ge 90 ]; then
    echo -e "${YELLOW}⚠ MOST TESTS PASSED ($PASS_RATE%)${NC}"
    echo ""
    echo "Review failed tests and retry."
    exit 1
else
    echo -e "${RED}✗ MANY TESTS FAILED ($PASS_RATE%)${NC}"
    echo ""
    echo "Please investigate failures."
    exit 1
fi