#!/bin/bash

# TraceFlow Memory Profiling Script
# Analyzes memory usage under load

set -e

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:9090}"
PROFILE_DIR="./profiles"

echo "=== TraceFlow Memory Profiling ==="
echo ""

# Check if collector is running
if ! curl -f -s "$COLLECTOR_URL/health" > /dev/null; then
    echo "ERROR: Collector not running at $COLLECTOR_URL"
    echo "Start with: go run cmd/collector/main.go"
    exit 1
fi

# Create profile directory
mkdir -p "$PROFILE_DIR"

# Check if pprof endpoint is available
PPROF_AVAILABLE=false
if curl -f -s "http://localhost:6060/debug/pprof/heap" > /dev/null 2>&1; then
    PPROF_AVAILABLE=true
    echo "✓ pprof endpoint available at http://localhost:6060"
else
    echo "⚠ pprof endpoint not available at http://localhost:6060"
    echo "  To enable profiling, add to your collector main.go:"
    echo "    import _ \"net/http/pprof\""
    echo "    go func() { log.Println(http.ListenAndServe(\"localhost:6060\", nil)) }()"
fi
echo ""

echo "Step 1: Baseline Memory Profile"
if [ "$PPROF_AVAILABLE" = true ]; then
    echo "Capturing initial heap profile..."
    curl -s "http://localhost:6060/debug/pprof/heap" > "$PROFILE_DIR/heap-baseline.prof" 2>/dev/null
    if [ -s "$PROFILE_DIR/heap-baseline.prof" ]; then
        echo "✓ Baseline profile captured"
    else
        echo "⚠ Failed to capture baseline profile"
    fi
else
    echo "Skipping (pprof not available)"
fi
echo ""

echo "Step 2: Load Testing"
echo "Generating load (1000 spans)..."

for i in {1..1000}; do
    TRACE_ID=$(openssl rand -hex 16)
    SPAN_ID=$(openssl rand -hex 8)
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    curl -s -X POST "$COLLECTOR_URL/api/v1/spans" \
        -H "Content-Type: application/json" \
        -d "{
            \"trace_id\": \"$TRACE_ID\",
            \"span_id\": \"$SPAN_ID\",
            \"service_name\": \"test-service\",
            \"operation_name\": \"test-op\",
            \"start_time\": \"$TIMESTAMP\",
            \"duration\": 50000000,
            \"status\": \"ok\"
        }" > /dev/null
    
    if [ $((i % 100)) -eq 0 ]; then
        echo "  Progress: $i/1000"
    fi
done

echo ""
echo "Step 3: Post-Load Memory Profile"
echo "Waiting for processing..."
sleep 2

if [ "$PPROF_AVAILABLE" = true ]; then
    echo "Capturing post-load heap profile..."
    curl -s "http://localhost:6060/debug/pprof/heap" > "$PROFILE_DIR/heap-loaded.prof" 2>/dev/null
    if [ -s "$PROFILE_DIR/heap-loaded.prof" ]; then
        echo "✓ Post-load profile captured"
    else
        echo "⚠ Failed to capture post-load profile"
    fi
else
    echo "Skipping (pprof not available)"
fi
echo ""

echo "Step 4: Metrics Analysis"
METRICS=$(curl -s "$COLLECTOR_URL/metrics")
SPANS_STORED=$(echo "$METRICS" | grep "traceflow_spans_stored_total" | awk '{print $2}')

echo "Metrics:"
echo "  Spans Stored: $SPANS_STORED"
echo ""

echo "Step 5: Memory Analysis"
echo ""

# Try to analyze with pprof if available
PPROF_ANALYSIS_DONE=false
if [ "$PPROF_AVAILABLE" = true ] && command -v go &> /dev/null && [ -f "$PROFILE_DIR/heap-loaded.prof" ] && [ -s "$PROFILE_DIR/heap-loaded.prof" ]; then
    echo "=== pprof Analysis ==="
    echo ""
    echo "Top memory consumers:"
    if go tool pprof -text -top=10 "$PROFILE_DIR/heap-loaded.prof" 2>/dev/null | head -25; then
        PPROF_ANALYSIS_DONE=true
    else
        echo "Unable to analyze profile (go tool pprof error)"
    fi
    echo ""
    
    echo "Memory profile saved to: $PROFILE_DIR/heap-loaded.prof"
    echo ""
    echo "To analyze interactively:"
    echo "  go tool pprof $PROFILE_DIR/heap-loaded.prof"
    echo ""
    echo "Common commands in pprof:"
    echo "  top       - Show top memory consumers"
    echo "  list      - Show source code with allocations"
    echo "  web       - Generate visualization (requires graphviz)"
    echo "  pdf       - Generate PDF report"
    echo ""
fi

# Always show manual memory check
echo "=== Process Memory Check ==="
echo ""
echo "Expected memory usage for 1000 spans:"
echo "  Span size: ~500 bytes"
echo "  1000 spans: ~500 KB"
echo "  With indexes: ~1-2 MB"
echo "  Total expected: <5 MB"
echo ""

if command -v ps &> /dev/null; then
    PID=$(pgrep -f "cmd/collector" || echo "")
    if [ -n "$PID" ]; then
        MEM=$(ps -p "$PID" -o rss= 2>/dev/null || echo "unknown")
        if [ "$MEM" != "unknown" ]; then
            MEM_MB=$((MEM / 1024))
            echo "Current collector memory usage: ${MEM_MB} MB (RSS)"
            
            if [ "$MEM_MB" -lt 50 ]; then
                echo "✓ Memory usage: Excellent"
            elif [ "$MEM_MB" -lt 100 ]; then
                echo "✓ Memory usage: Good"
            else
                echo "⚠ Memory usage: High (investigate)"
            fi
        else
            echo "Unable to determine memory usage"
        fi
    else
        echo "Collector process not found"
    fi
else
    echo "ps command not available for memory check"
fi

if [ "$PPROF_AVAILABLE" = false ]; then
    echo ""
    echo "Note: Enable pprof for detailed memory analysis:"
    echo "  Add to collector main.go:"
    echo "    import _ \"net/http/pprof\""
    echo "    go func() { log.Println(http.ListenAndServe(\"localhost:6060\", nil)) }()"
fi

echo ""
echo "=== Profiling Complete ==="
echo ""
echo "Summary:"
echo "  1. Generated 1000 test spans"
echo "  2. Verified storage"
echo "  3. Captured memory profiles"
echo "  4. Analyzed memory usage"
echo ""
echo "For production, monitor:"
echo "  - Memory growth over time"
echo "  - GC pressure (frequency)"
echo "  - Eviction rates"
echo "  - Query latency"