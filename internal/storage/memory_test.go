package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
)

func TestWriteSpan_SingleSpan(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	span := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "test-service",
		OperationName: "test-op",
		StartTime:     time.Now(),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}

	err := store.WriteSpan(ctx, span)
	if err != nil {
		t.Fatalf("WriteSpan failed: %v", err)
	}

	// Verify span was stored
	value, ok := store.spans.Load(span.SpanID)
	if !ok {
		t.Fatal("span not found in storage")
	}

	storedSpan := value.(*models.Span)
	if storedSpan.SpanID != span.SpanID {
		t.Errorf("stored span ID = %s, want %s", storedSpan.SpanID, span.SpanID)
	}
}

func TestWriteSpan_InvalidSpan(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	// Missing required field
	span := &models.Span{
		TraceID: models.GenerateTraceID(),
		SpanID:  models.GenerateSpanID(),
		// Missing ServiceName - should fail validation
		OperationName: "test-op",
		StartTime:     time.Now(),
		Status:        "ok",
	}

	err := store.WriteSpan(ctx, span)
	if err == nil {
		t.Fatal("expected error for invalid span, got nil")
	}
}

func TestWriteSpan_Concurrent(t *testing.T) {
	store := NewMemoryStore(10000)
	ctx := context.Background()

	const goroutines = 100
	const spansPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*spansPerGoroutine)

	// Write spans concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < spansPerGoroutine; j++ {
				span := &models.Span{
					TraceID:       models.GenerateTraceID(),
					SpanID:        models.GenerateSpanID(),
					ServiceName:   "test-service",
					OperationName: "test-op",
					StartTime:     time.Now(),
					Duration:      time.Duration(routineID+j) * time.Millisecond,
					Status:        "ok",
				}

				if err := store.WriteSpan(ctx, span); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}

	// Verify all spans were stored
	count := 0
	store.spans.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	expected := goroutines * spansPerGoroutine
	if count != expected {
		t.Errorf("stored %d spans, want %d", count, expected)
	}
}

func TestGetTrace_AssembleCorrectly(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	traceID := models.GenerateTraceID()
	now := time.Now()

	// Create a trace with 3 spans
	spans := []*models.Span{
		{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "frontend",
			OperationName: "page-load",
			StartTime:     now,
			Duration:      100 * time.Millisecond,
			Status:        "ok",
			SpanKind:      "client",
		},
		{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ParentSpanID:  "parent-span-id", // Not the actual parent, but that's ok
			ServiceName:   "api",
			OperationName: "get-users",
			StartTime:     now.Add(10 * time.Millisecond),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
			SpanKind:      "server",
		},
		{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ParentSpanID:  "parent-span-id",
			ServiceName:   "database",
			OperationName: "query",
			StartTime:     now.Add(20 * time.Millisecond),
			Duration:      25 * time.Millisecond,
			Status:        "ok",
			SpanKind:      "client",
		},
	}

	// Store all spans
	for _, span := range spans {
		if err := store.WriteSpan(ctx, span); err != nil {
			t.Fatalf("WriteSpan failed: %v", err)
		}
	}

	// Retrieve trace
	trace, err := store.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}
	if trace == nil {
		t.Fatal("trace is nil")
	}

	// Verify trace metadata
	if trace.TraceID != traceID {
		t.Errorf("trace ID = %s, want %s", trace.TraceID, traceID)
	}

	if len(trace.Spans) != 3 {
		t.Errorf("trace has %d spans, want 3", len(trace.Spans))
	}

	if len(trace.Services) != 3 {
		t.Errorf("trace has %d services, want 3", len(trace.Services))
	}

	// Verify services are sorted
	expectedServices := []string{"api", "database", "frontend"}
	for i, service := range expectedServices {
		if trace.Services[i] != service {
			t.Errorf("service[%d] = %s, want %s", i, trace.Services[i], service)
		}
	}

	// Verify duration calculation
	// Trace duration should be from earliest start to latest end
	// Frontend: 0ms -> 100ms
	// API: 10ms -> 60ms
	// Database: 20ms -> 45ms
	// Total: 0ms -> 100ms = 100ms
	expectedDuration := 100 * time.Millisecond
	if trace.Duration < expectedDuration-time.Millisecond || trace.Duration > expectedDuration+time.Millisecond {
		t.Errorf("trace duration = %v, want ~%v", trace.Duration, expectedDuration)
	}
}

func TestGetTrace_NotFound(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	trace, err := store.GetTrace(ctx, "nonexistent-trace-id")
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}
	if trace != nil {
		t.Error("expected nil trace for nonexistent ID")
	}
}

func TestFindTraces_FilterByService(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	// Create traces with different services
	createTestTrace(t, store, "frontend", 50*time.Millisecond)
	createTestTrace(t, store, "api", 100*time.Millisecond)
	createTestTrace(t, store, "frontend", 75*time.Millisecond)

	// Query for frontend traces
	query := NewQuery().WithService("frontend")
	traces, err := store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}

	if len(traces) != 2 {
		t.Errorf("found %d traces, want 2", len(traces))
	}

	// Verify all returned traces include the frontend service
	for _, trace := range traces {
		found := false
		for _, service := range trace.Services {
			if service == "frontend" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("trace %s does not include frontend service", trace.TraceID)
		}
	}
}

func TestFindTraces_FilterByDuration(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	// Create traces with different durations
	createTestTrace(t, store, "api", 25*time.Millisecond)
	createTestTrace(t, store, "api", 75*time.Millisecond)
	createTestTrace(t, store, "api", 150*time.Millisecond)

	// Query for traces between 50ms and 100ms
	query := NewQuery().WithDurationRange(50*time.Millisecond, 100*time.Millisecond)
	traces, err := store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}

	if len(traces) != 1 {
		t.Errorf("found %d traces, want 1", len(traces))
	}

	if len(traces) > 0 {
		if traces[0].Duration < 50*time.Millisecond || traces[0].Duration > 100*time.Millisecond {
			t.Errorf("trace duration %v not in range [50ms, 100ms]", traces[0].Duration)
		}
	}
}

func TestFindTraces_FilterByTimeRange(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	now := time.Now()

	// Create spans at different times
	span1 := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "api",
		OperationName: "test",
		StartTime:     now.Add(-2 * time.Hour),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}
	span2 := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "api",
		OperationName: "test",
		StartTime:     now.Add(-30 * time.Minute),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}
	span3 := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "api",
		OperationName: "test",
		StartTime:     now,
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}

	store.WriteSpan(ctx, span1)
	store.WriteSpan(ctx, span2)
	store.WriteSpan(ctx, span3)

	// Query for traces in last hour
	query := NewQuery().WithTimeRange(now.Add(-1*time.Hour), now.Add(1*time.Hour))
	traces, err := store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}

	// Should find span2 and span3 (both within last hour)
	if len(traces) != 2 {
		t.Errorf("found %d traces, want 2", len(traces))
	}
}

func TestFindTraces_Pagination(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	// Create 10 traces
	for i := 0; i < 10; i++ {
		createTestTrace(t, store, "api", time.Duration(i+1)*10*time.Millisecond)
	}

	// First page (limit 5, offset 0)
	query := NewQuery().WithPagination(5, 0)
	traces, err := store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}
	if len(traces) != 5 {
		t.Errorf("page 1: found %d traces, want 5", len(traces))
	}

	// Second page (limit 5, offset 5)
	query = NewQuery().WithPagination(5, 5)
	traces, err = store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}
	if len(traces) != 5 {
		t.Errorf("page 2: found %d traces, want 5", len(traces))
	}

	// Page beyond results (offset 20)
	query = NewQuery().WithPagination(5, 20)
	traces, err = store.FindTraces(ctx, query)
	if err != nil {
		t.Fatalf("FindTraces failed: %v", err)
	}
	if len(traces) != 0 {
		t.Errorf("beyond results: found %d traces, want 0", len(traces))
	}
}

func TestGetServices(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	// Create traces with different services
	createTestTrace(t, store, "frontend", 50*time.Millisecond)
	createTestTrace(t, store, "api", 50*time.Millisecond)
	createTestTrace(t, store, "database", 50*time.Millisecond)
	createTestTrace(t, store, "api", 50*time.Millisecond) // Duplicate service

	services, err := store.GetServices(ctx)
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}

	if len(services) != 3 {
		t.Errorf("found %d services, want 3", len(services))
	}

	// Verify services are sorted
	expectedServices := []string{"api", "database", "frontend"}
	for i, expected := range expectedServices {
		if services[i] != expected {
			t.Errorf("service[%d] = %s, want %s", i, services[i], expected)
		}
	}
}

func TestEviction(t *testing.T) {
	// Create store with small capacity
	store := NewMemoryStore(5)

	// Create 10 traces (should trigger eviction)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond) // Ensure different timestamps
		createTestTrace(t, store, "api", 50*time.Millisecond)
	}

	// Count traces
	count := 0
	store.traces.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count > 5 {
		t.Errorf("stored %d traces, want <= 5 (eviction failed)", count)
	}
}

func TestIndexing_ServiceIndex(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	traceID := models.GenerateTraceID()
	span := &models.Span{
		TraceID:       traceID,
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "test-service",
		OperationName: "test-op",
		StartTime:     time.Now(),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}

	store.WriteSpan(ctx, span)

	// Check service index
	store.indexMu.RLock()
	traceIDs := store.indexes.byService["test-service"]
	store.indexMu.RUnlock()

	if len(traceIDs) != 1 {
		t.Errorf("service index has %d traces, want 1", len(traceIDs))
	}
	if traceIDs[0] != traceID {
		t.Errorf("service index trace ID = %s, want %s", traceIDs[0], traceID)
	}
}

func TestIndexing_TimestampBuckets(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	now := time.Now()
	traceID := models.GenerateTraceID()
	span := &models.Span{
		TraceID:       traceID,
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "test-service",
		OperationName: "test-op",
		StartTime:     now,
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}

	store.WriteSpan(ctx, span)

	// Check time bucket
	hourBucket := now.Unix() / 3600
	store.indexMu.RLock()
	traceIDs := store.indexes.byTimestamp.buckets[hourBucket]
	store.indexMu.RUnlock()

	if len(traceIDs) != 1 {
		t.Errorf("time bucket has %d traces, want 1", len(traceIDs))
	}
	if traceIDs[0] != traceID {
		t.Errorf("time bucket trace ID = %s, want %s", traceIDs[0], traceID)
	}
}

func TestIndexing_DurationBuckets(t *testing.T) {
	store := NewMemoryStore(1000)
	ctx := context.Background()

	tests := []struct {
		name     string
		duration time.Duration
		bucket   string
	}{
		{"fast", 5 * time.Millisecond, "fast"},
		{"medium", 50 * time.Millisecond, "medium"},
		{"slow", 500 * time.Millisecond, "slow"},
		{"verySlow", 2000 * time.Millisecond, "verySlow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID := models.GenerateTraceID()
			span := &models.Span{
				TraceID:       traceID,
				SpanID:        models.GenerateSpanID(),
				ServiceName:   "test-service",
				OperationName: "test-op",
				StartTime:     time.Now(),
				Duration:      tt.duration,
				Status:        "ok",
			}

			store.WriteSpan(ctx, span)

			// Check appropriate bucket
			store.indexMu.RLock()
			var found bool
			switch tt.bucket {
			case "fast":
				found = store.containsString(store.indexes.byDuration.fast, traceID)
			case "medium":
				found = store.containsString(store.indexes.byDuration.medium, traceID)
			case "slow":
				found = store.containsString(store.indexes.byDuration.slow, traceID)
			case "verySlow":
				found = store.containsString(store.indexes.byDuration.verySlow, traceID)
			}
			store.indexMu.RUnlock()

			if !found {
				t.Errorf("trace not found in %s bucket", tt.bucket)
			}
		})
	}
}

// Helper function to create a simple test trace
func createTestTrace(t *testing.T, store *MemoryStore, serviceName string, duration time.Duration) string {
	t.Helper()

	traceID := models.GenerateTraceID()
	span := &models.Span{
		TraceID:       traceID,
		SpanID:        models.GenerateSpanID(),
		ServiceName:   serviceName,
		OperationName: "test-op",
		StartTime:     time.Now(),
		Duration:      duration,
		Status:        "ok",
	}

	if err := store.WriteSpan(context.Background(), span); err != nil {
		t.Fatalf("failed to create test trace: %v", err)
	}

	return traceID
}
