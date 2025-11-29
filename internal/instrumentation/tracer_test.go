package instrumentation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Mock collector server for testing
func mockCollector(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/spans" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
}

func TestNewTracer(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")

	if tracer.serviceName != "test-service" {
		t.Errorf("serviceName = %s, want test-service", tracer.serviceName)
	}
	if tracer.collectorUrl != "http://localhost:9090" {
		t.Errorf("collectorUrl = %s, want http://localhost:9090", tracer.collectorUrl)
	}
	if tracer.client == nil {
		t.Error("client is nil")
	}
	if tracer.sampler == nil {
		t.Error("sampler is nil")
	}
}

func TestStartSpan_CreatesSpan(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	span, ctx := tracer.StartSpan(ctx, "test-operation")

	if span == nil {
		t.Fatal("span is nil")
	}
	if span.span == nil {
		t.Fatal("span.span is nil")
	}
	if span.span.ServiceName != "test-service" {
		t.Errorf("ServiceName = %s, want test-service", span.span.ServiceName)
	}
	if span.span.OperationName != "test-operation" {
		t.Errorf("OperationName = %s, want test-operation", span.span.OperationName)
	}
	if span.span.TraceID == "" {
		t.Error("TraceID is empty")
	}
	if span.span.SpanID == "" {
		t.Error("SpanID is empty")
	}
}

func TestStartSpan_WithParent(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	// Create parent span
	parent, ctx := tracer.StartSpan(ctx, "parent-operation")
	parentTraceID := parent.span.TraceID
	parentSpanID := parent.span.SpanID

	// Create child span
	child, _ := tracer.StartSpan(ctx, "child-operation")

	if child.span.TraceID != parentTraceID {
		t.Errorf("child TraceID = %s, want %s", child.span.TraceID, parentTraceID)
	}
	if child.span.ParentSpanID != parentSpanID {
		t.Errorf("child ParentSpanID = %s, want %s", child.span.ParentSpanID, parentSpanID)
	}
}

func TestSpan_SetTag(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	span, _ := tracer.StartSpan(ctx, "test-operation")
	span.SetTag("key1", "value1")
	span.SetTag("key2", "value2")

	if span.span.Tags["key1"] != "value1" {
		t.Errorf("tag key1 = %s, want value1", span.span.Tags["key1"])
	}
	if span.span.Tags["key2"] != "value2" {
		t.Errorf("tag key2 = %s, want value2", span.span.Tags["key2"])
	}
}

func TestSpan_SetError(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	span, _ := tracer.StartSpan(ctx, "test-operation")
	testErr := errors.New("test error")
	span.SetError(testErr)

	if span.span.Status != "error" {
		t.Errorf("Status = %s, want error", span.span.Status)
	}
	if span.span.StatusMessage != "test error" {
		t.Errorf("StatusMessage = %s, want test error", span.span.StatusMessage)
	}
	if span.span.Tags["error"] != "true" {
		t.Errorf("error tag = %s, want true", span.span.Tags["error"])
	}
}

func TestSpan_Finish(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	ctx := context.Background()

	span, _ := tracer.StartSpan(ctx, "test-operation")

	// Sleep briefly to ensure duration > 0
	time.Sleep(10 * time.Millisecond)

	span.Finish()

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	if span.span.Duration == 0 {
		t.Error("Duration is 0")
	}
}

func TestWithTags(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	tags := map[string]string{
		"tag1": "value1",
		"tag2": "value2",
	}

	span, _ := tracer.StartSpan(ctx, "test-operation", WithTags(tags))

	if span.span.Tags["tag1"] != "value1" {
		t.Errorf("tag1 = %s, want value1", span.span.Tags["tag1"])
	}
	if span.span.Tags["tag2"] != "value2" {
		t.Errorf("tag2 = %s, want value2", span.span.Tags["tag2"])
	}
}

func TestWithSpanKind(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	span, _ := tracer.StartSpan(ctx, "test-operation", WithSpanKind("client"))

	if span.span.SpanKind != "client" {
		t.Errorf("SpanKind = %s, want client", span.span.SpanKind)
	}
}

// Trace Context Tests

func TestEncodeTraceParent(t *testing.T) {
	traceID := "0af7651916cd43dd8448eb211c80319c"
	spanID := "b7ad6b7169203331"
	flags := "01"

	result := EncodeTraceParent(traceID, spanID, flags)
	expected := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	if result != expected {
		t.Errorf("EncodeTraceParent() = %s, want %s", result, expected)
	}
}

func TestEncodeTraceParent_DefaultFlags(t *testing.T) {
	traceID := "0af7651916cd43dd8448eb211c80319c"
	spanID := "b7ad6b7169203331"

	result := EncodeTraceParent(traceID, spanID, "")
	expected := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	if result != expected {
		t.Errorf("EncodeTraceParent() = %s, want %s", result, expected)
	}
}

func TestDecodeTraceParent_Valid(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	tc, err := DecodeTraceParent(header)
	if err != nil {
		t.Fatalf("DecodeTraceParent() error = %v", err)
	}

	if tc.Version != "00" {
		t.Errorf("Version = %s, want 00", tc.Version)
	}
	if tc.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("TraceID = %s, want 0af7651916cd43dd8448eb211c80319c", tc.TraceID)
	}
	if tc.SpanID != "b7ad6b7169203331" {
		t.Errorf("SpanID = %s, want b7ad6b7169203331", tc.SpanID)
	}
	if tc.Flags != "01" {
		t.Errorf("Flags = %s, want 01", tc.Flags)
	}
}

func TestDecodeTraceParent_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"empty", ""},
		{"invalid format", "invalid-header"},
		{"short trace ID", "00-0af7651916cd43dd8448eb211c8031-b7ad6b7169203331-01"},
		{"short span ID", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b716920333-01"},
		{"missing parts", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeTraceParent(tt.header)
			if err == nil {
				t.Error("DecodeTraceParent() expected error, got nil")
			}
		})
	}
}

func TestIsValidTraceParent(t *testing.T) {
	tests := []struct {
		header string
		valid  bool
	}{
		{"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", true},
		{"00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-00", true},
		{"invalid", false},
		{"", false},
		{"00-short-b7ad6b7169203331-01", false},
	}

	for _, tt := range tests {
		result := IsValidTraceParent(tt.header)
		if result != tt.valid {
			t.Errorf("IsValidTraceParent(%s) = %v, want %v", tt.header, result, tt.valid)
		}
	}
}

// Context Tests

func TestSpanFromContext(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	span, ctx := tracer.StartSpan(ctx, "test-operation")

	retrieved := SpanFromContext(ctx)
	if retrieved == nil {
		t.Fatal("SpanFromContext() returned nil")
	}
	if retrieved.span.SpanID != span.span.SpanID {
		t.Errorf("retrieved span ID = %s, want %s", retrieved.span.SpanID, span.span.SpanID)
	}
}

func TestSpanFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()

	span := SpanFromContext(ctx)
	if span != nil {
		t.Error("SpanFromContext() should return nil when no span in context")
	}
}

// Middleware Tests

func TestMiddleware_CreatesSpan(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	middleware := Middleware(tracer)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check span is in context
		span := SpanFromContext(r.Context())
		if span == nil {
			t.Error("span not in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Wait for async span send
	time.Sleep(100 * time.Millisecond)
}

func TestMiddleware_SetsHTTPTags(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	middleware := Middleware(tracer)

	var capturedSpan *Span

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpan = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedSpan == nil {
		t.Fatal("span is nil")
	}

	if capturedSpan.span.Tags["http.method"] != "GET" {
		t.Errorf("http.method = %s, want GET", capturedSpan.span.Tags["http.method"])
	}
	if capturedSpan.span.Tags["http.url"] != "/test/path" {
		t.Errorf("http.url = %s, want /test/path", capturedSpan.span.Tags["http.url"])
	}
}

func TestMiddleware_PropagatesTraceContext(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	middleware := Middleware(tracer)

	traceID := "0af7651916cd43dd8448eb211c80319c"
	spanID := "b7ad6b7169203331"
	traceparent := EncodeTraceParent(traceID, spanID, "01")

	var capturedSpan *Span

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpan = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(TraceParentHeader, traceparent)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedSpan == nil {
		t.Fatal("span is nil")
	}

	// Should use trace ID from header
	if capturedSpan.span.TraceID != traceID {
		t.Errorf("TraceID = %s, want %s", capturedSpan.span.TraceID, traceID)
	}
	// Parent span ID should be the span ID from header
	if capturedSpan.span.ParentSpanID != spanID {
		t.Errorf("ParentSpanID = %s, want %s", capturedSpan.span.ParentSpanID, spanID)
	}
}

func TestMiddleware_CapturesStatusCode(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	middleware := Middleware(tracer)

	var capturedSpan *Span

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpan = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedSpan == nil {
		t.Fatal("span is nil")
	}

	// Wait for middleware to finish
	time.Sleep(10 * time.Millisecond)

	if capturedSpan.span.Tags["http.status_code"] != "404" {
		t.Errorf("http.status_code = %s, want 404", capturedSpan.span.Tags["http.status_code"])
	}
}

func TestMiddleware_MarksErrorOn500(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)
	middleware := Middleware(tracer)

	var capturedSpan *Span

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSpan = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedSpan == nil {
		t.Fatal("span is nil")
	}

	// Wait for middleware to finish
	time.Sleep(10 * time.Millisecond)

	if capturedSpan.span.Status != "error" {
		t.Errorf("Status = %s, want error", capturedSpan.span.Status)
	}
}

// HTTP Client Tests

func TestWrapHTTPClient_InjectsTraceContext(t *testing.T) {
	tracer := NewTracer("test-service", "http://localhost:9090")
	ctx := context.Background()

	// Start a span
	span, ctx := tracer.StartSpan(ctx, "test-operation")

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for traceparent header
		traceparent := r.Header.Get(TraceParentHeader)
		if traceparent == "" {
			t.Error("traceparent header not set")
		}

		// Decode and verify
		tc, err := DecodeTraceParent(traceparent)
		if err != nil {
			t.Errorf("invalid traceparent: %v", err)
		}
		if tc.TraceID != span.span.TraceID {
			t.Errorf("trace ID mismatch: got %s, want %s", tc.TraceID, span.span.TraceID)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Wrap client
	client := WrapHTTPClient(http.DefaultClient)

	// Make request
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestClientMiddleware_CreatesSpan(t *testing.T) {
	server := mockCollector(t)
	defer server.Close()

	tracer := NewTracer("test-service", server.URL)

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Apply client middleware
	client := ClientMiddleware(tracer)(http.DefaultClient)

	// Make request
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// Wait for async span send
	time.Sleep(100 * time.Millisecond)
}
