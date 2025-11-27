package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
	"github.com/saintparish4/asmbly/internal/storage"
)

func TestHandlePostSpan_Success(t *testing.T) {
	// Setup
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Create test span
	span := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "test-service",
		OperationName: "test-op",
		StartTime:     time.Now(),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}

	spanJSON, _ := json.Marshal(span)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spans", bytes.NewReader(spanJSON))
	rec := httptest.NewRecorder()

	// Execute
	col.HandlePostSpan(rec, req)

	// Verify response
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify span was stored
	trace, err := store.GetTrace(ctx, span.TraceID)
	if err != nil {
		t.Fatalf("failed to get trace: %v", err)
	}
	if trace == nil {
		t.Fatal("trace not found")
	}
	if len(trace.Spans) != 1 {
		t.Errorf("trace has %d spans, want 1", len(trace.Spans))
	}
}

func TestHandlePostSpan_InvalidJSON(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spans", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	col.HandlePostSpan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlePostSpan_InvalidSpan(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Invalid span (missing required fields)
	span := &models.Span{
		TraceID: models.GenerateTraceID(),
		SpanID:  models.GenerateSpanID(),
		// Missing ServiceName - invalid
	}

	spanJSON, _ := json.Marshal(span)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spans", bytes.NewReader(spanJSON))
	rec := httptest.NewRecorder()

	col.HandlePostSpan(rec, req)

	// Should accept (202) but worker will fail to store
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check metrics - should have error
	metrics := col.GetMetrics()
	if metrics.SpanErrors == 0 {
		t.Error("expected span error, got none")
	}
}

func TestHandlePostSpansBatch_Success(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 100}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Create test spans
	traceID := models.GenerateTraceID()
	spans := []models.Span{
		{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "frontend",
			OperationName: "page-load",
			StartTime:     time.Now(),
			Duration:      100 * time.Millisecond,
			Status:        "ok",
		},
		{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "api",
			OperationName: "get-users",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		},
	}

	spansJSON, _ := json.Marshal(spans)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spans/batch", bytes.NewReader(spansJSON))
	rec := httptest.NewRecorder()

	// Execute
	col.HandlePostSpansBatch(rec, req)

	// Verify response
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Parse response
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	if int(result["accepted"].(float64)) != 2 {
		t.Errorf("accepted = %v, want 2", result["accepted"])
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify trace was stored
	trace, err := store.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatalf("failed to get trace: %v", err)
	}
	if trace == nil {
		t.Fatal("trace not found")
	}
	if len(trace.Spans) != 2 {
		t.Errorf("trace has %d spans, want 2", len(trace.Spans))
	}
}

func TestHandleGetTrace_Found(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()

	// Create and store a span
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

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/"+traceID, nil)
	rec := httptest.NewRecorder()

	// Execute
	col.HandleGetTrace(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse response
	var trace models.Trace
	if err := json.NewDecoder(rec.Body).Decode(&trace); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if trace.TraceID != traceID {
		t.Errorf("trace_id = %s, want %s", trace.TraceID, traceID)
	}
	if len(trace.Spans) != 1 {
		t.Errorf("spans = %d, want 1", len(trace.Spans))
	}
}

func TestHandleGetTrace_NotFound(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	// Create request with nonexistent trace ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/nonexistent", nil)
	rec := httptest.NewRecorder()

	// Execute
	col.HandleGetTrace(rec, req)

	// Verify response
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleFindTraces_WithFilters(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()

	// Create test spans with different services
	services := []string{"frontend", "api", "database"}
	for _, service := range services {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   service,
			OperationName: "test-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		store.WriteSpan(ctx, span)
	}

	// Query for "frontend" service
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces?service=frontend", nil)
	rec := httptest.NewRecorder()

	col.HandleFindTraces(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse response
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	traces := result["traces"].([]interface{})
	if len(traces) != 1 {
		t.Errorf("found %d traces, want 1", len(traces))
	}
}

func TestHandleFindTraces_Pagination(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()

	// Create 10 test spans
	for i := 0; i < 10; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "test-service",
			OperationName: "test-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		store.WriteSpan(ctx, span)
	}

	// Query with pagination
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces?limit=5&offset=0", nil)
	rec := httptest.NewRecorder()

	col.HandleFindTraces(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse response
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	traces := result["traces"].([]interface{})
	if len(traces) != 5 {
		t.Errorf("found %d traces, want 5", len(traces))
	}
}

func TestHandleGetServices(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()

	// Create spans for different services
	services := []string{"frontend", "api", "database"}
	for _, service := range services {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   service,
			OperationName: "test-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		store.WriteSpan(ctx, span)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	rec := httptest.NewRecorder()

	// Execute
	col.HandleGetServices(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse response
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	servicesList := result["services"].([]interface{})
	if len(servicesList) != 3 {
		t.Errorf("found %d services, want 3", len(servicesList))
	}
}

func TestIntegration_SubmitAndRetrieve(t *testing.T) {
	// This is an end-to-end test: submit span → retrieve trace

	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 10}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// 1. Submit a span
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

	spanJSON, _ := json.Marshal(span)
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/spans", bytes.NewReader(spanJSON))
	postRec := httptest.NewRecorder()

	col.HandlePostSpan(postRec, postReq)

	if postRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/spans status = %d, want %d", postRec.Code, http.StatusAccepted)
	}

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	// 2. Retrieve the trace
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/traces/"+traceID, nil)
	getRec := httptest.NewRecorder()

	col.HandleGetTrace(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/traces/:id status = %d, want %d", getRec.Code, http.StatusOK)
	}

	// Verify trace
	var trace models.Trace
	if err := json.NewDecoder(getRec.Body).Decode(&trace); err != nil {
		t.Fatalf("failed to decode trace: %v", err)
	}

	if trace.TraceID != traceID {
		t.Errorf("trace_id = %s, want %s", trace.TraceID, traceID)
	}
	if len(trace.Spans) != 1 {
		t.Errorf("spans = %d, want 1", len(trace.Spans))
	}
}

func TestWorkerPool_ConcurrentProcessing(t *testing.T) {
	store := storage.NewMemoryStore(10000)
	config := &Config{Workers: 5, ChannelBuffer: 1000}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Submit many spans concurrently
	const numSpans = 100
	for i := 0; i < numSpans; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "test-service",
			OperationName: "test-op",
			StartTime:     time.Now(),
			Duration:      time.Duration(i) * time.Millisecond,
			Status:        "ok",
		}

		if err := col.SubmitSpan(span); err != nil {
			t.Fatalf("failed to submit span: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify metrics
	metrics := col.GetMetrics()
	if metrics.SpansReceived != numSpans {
		t.Errorf("spans_received = %d, want %d", metrics.SpansReceived, numSpans)
	}
	if metrics.SpansStored != numSpans {
		t.Errorf("spans_stored = %d, want %d", metrics.SpansStored, numSpans)
	}
}

func TestGracefulShutdown(t *testing.T) {
	store := storage.NewMemoryStore(1000)
	config := &Config{Workers: 2, ChannelBuffer: 100}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)

	// Submit spans
	for i := 0; i < 10; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "test-service",
			OperationName: "test-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		col.SubmitSpan(span)
	}

	// Shutdown (should wait for in-flight spans)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := col.Stop(shutdownCtx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}

	// Verify all spans were processed
	metrics := col.GetMetrics()
	if metrics.SpansReceived != metrics.SpansStored {
		t.Errorf("shutdown lost spans: received=%d, stored=%d",
			metrics.SpansReceived, metrics.SpansStored)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Verify CORS headers
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS origin header not set")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("CORS methods header not set")
	}
}

// Benchmark span submission throughput
func BenchmarkSubmitSpan(b *testing.B) {
	store := storage.NewMemoryStore(100000)
	config := &Config{Workers: 10, ChannelBuffer: 10000}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Pre-generate spans
	spans := make([]*models.Span, b.N)
	for i := 0; i < b.N; i++ {
		spans[i] = &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "bench-service",
			OperationName: "bench-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		col.SubmitSpan(spans[i])
	}
}

// Benchmark HTTP endpoint
func BenchmarkHandlePostSpan(b *testing.B) {
	store := storage.NewMemoryStore(100000)
	config := &Config{Workers: 10, ChannelBuffer: 10000}
	col := NewCollector(store, config, slog.Default())

	ctx := context.Background()
	col.Start(ctx)
	defer col.Stop(ctx)

	// Pre-generate span JSON
	span := &models.Span{
		TraceID:       models.GenerateTraceID(),
		SpanID:        models.GenerateSpanID(),
		ServiceName:   "bench-service",
		OperationName: "bench-op",
		StartTime:     time.Now(),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
	}
	spanJSON, _ := json.Marshal(span)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/spans", bytes.NewReader(spanJSON))
		rec := httptest.NewRecorder()
		col.HandlePostSpan(rec, req)
	}
}
