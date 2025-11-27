#!/bin/bash

# TraceFlow Load Testing Script
# Tests the collector's ability to handle high throughput

set -e

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:9090}"
TOTAL_REQUESTS="${TOTAL_REQUESTS:-10000}"
CONCURRENCY="${CONCURRENCY:-100}"

echo "=== TraceFlow Load Test ==="
echo "Collector URL: $COLLECTOR_URL"
echo "Total Requests: $TOTAL_REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo ""

# Check if collector is running
echo "Checking if collector is running..."
if ! curl -f -s "$COLLECTOR_URL/health" > /dev/null; then
    echo "ERROR: Collector not responding at $COLLECTOR_URL"
    echo "Please start the collector with: go run cmd/collector/main.go"
    exit 1
fi
echo "✓ Collector is healthy"
echo ""

# Generate test span payload
TRACE_ID=$(openssl rand -hex 16)
SPAN_ID=$(openssl rand -hex 8)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

SPAN_JSON=$(cat <<EOF
{
  "trace_id": "$TRACE_ID",
  "span_id": "$SPAN_ID",
  "service_name": "load-test",
  "operation_name": "test-operation",
  "start_time": "$TIMESTAMP",
  "duration": 50000000,
  "status": "ok",
  "tags": {
    "test": "load",
    "environment": "test"
  }
}
EOF
)

echo "Test payload:"
echo "$SPAN_JSON"
echo ""

# Save payload to temp file
PAYLOAD_FILE=$(mktemp)
echo "$SPAN_JSON" > "$PAYLOAD_FILE"

# Check if hey is installed
if ! command -v hey &> /dev/null; then
    echo "Installing 'hey' load testing tool..."
    
    # Try with direct connection if proxy fails
    if ! go install github.com/rakyll/hey@latest 2>/dev/null; then
        echo "⚠ Installation via Go proxy failed, trying direct connection..."
        GOPROXY=direct go install github.com/rakyll/hey@latest 2>/dev/null || true
    fi
    
    if ! command -v hey &> /dev/null; then
        echo "⚠ 'hey' not available. Using curl fallback for load testing..."
        USE_CURL_FALLBACK=true
    else
        echo "✓ 'hey' installed successfully"
        USE_CURL_FALLBACK=false
    fi
else
    USE_CURL_FALLBACK=false
fi

echo "Running load test..."
echo ""

# Run load test
if [ "$USE_CURL_FALLBACK" = true ]; then
    echo "Using curl-based load test (slower but works without hey)..."
    echo "Sending $TOTAL_REQUESTS requests with concurrency=$CONCURRENCY"
    echo ""
    
    START_TIME=$(date +%s)
    SUCCESS=0
    FAILED=0
    
    # Run requests in background with concurrency limit
    for i in $(seq 1 "$TOTAL_REQUESTS"); do
        # Wait if we've hit concurrency limit
        while [ $(jobs -r | wc -l) -ge "$CONCURRENCY" ]; do
            sleep 0.01
        done
        
        # Send request in background
        (
            if curl -s -X POST \
                -H "Content-Type: application/json" \
                -d @"$PAYLOAD_FILE" \
                "$COLLECTOR_URL/api/v1/spans" \
                -o /dev/null -w "%{http_code}" | grep -q "202"; then
                echo "success" > /dev/null
            else
                echo "failed" > /dev/null
            fi
        ) &
        
        # Progress indicator
        if [ $((i % 100)) -eq 0 ]; then
            echo -n "."
        fi
    done
    
    # Wait for all background jobs to complete
    wait
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    RPS=$((TOTAL_REQUESTS / DURATION))
    
    echo ""
    echo ""
    echo "Summary:"
    echo "  Total requests: $TOTAL_REQUESTS"
    echo "  Duration: ${DURATION}s"
    echo "  Requests/sec: $RPS"
    echo ""
else
    # Use hey for proper load testing
    hey -n "$TOTAL_REQUESTS" \
        -c "$CONCURRENCY" \
        -m POST \
        -H "Content-Type: application/json" \
        -D "$PAYLOAD_FILE" \
        "$COLLECTOR_URL/api/v1/spans"
fi

# Clean up
rm "$PAYLOAD_FILE"

echo ""
echo "=== Checking Metrics ==="
curl -s "$COLLECTOR_URL/metrics" | grep traceflow_spans

echo ""
echo "=== Load Test Complete ==="
echo ""
echo "Expected throughput: >5,000 requests/sec"
echo "Check the summary above to verify performance."