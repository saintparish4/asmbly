package models

import (
	"sync"
	"testing"
	"time"
)

// TestSpanValidation_ValidSpan verifies that a properly formed span passes validation.
func TestSpanValidation_ValidSpan(t *testing.T) {
	span := &Span{
		TraceID:       GenerateTraceID(),
		SpanID:        GenerateSpanID(),
		ParentSpanID:  GenerateSpanID(),
		ServiceName:   "api-server",
		OperationName: "GET /users",
		StartTime:     time.Now(),
		Duration:      50 * time.Millisecond,
		Status:        "ok",
		SpanKind:      "server",
		Tags:          map[string]string{"http.method": "GET"},
	}

	if err := span.Validate(); err != nil {
		t.Errorf("valid span failed validation: %v", err)
	}
}

// TestSpanValidation_MissingRequiredFields tests all required field validations.
func TestSpanValidation_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		span        Span
		expectedErr error
	}{
		{
			name: "missing trace_id",
			span: Span{
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrMissingTraceID,
		},
		{
			name: "missing span_id",
			span: Span{
				TraceID:       GenerateTraceID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrMissingSpanID,
		},
		{
			name: "missing service_name",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrMissingServiceName,
		},
		{
			name: "missing operation_name",
			span: Span{
				TraceID:     GenerateTraceID(),
				SpanID:      GenerateSpanID(),
				ServiceName: "test",
				StartTime:   time.Now(),
				Status:      "ok",
			},
			expectedErr: ErrMissingOperationName,
		},
		{
			name: "missing start_time",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				Status:        "ok",
			},
			expectedErr: ErrMissingStartTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.span.Validate()
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// TestSpanValidation_InvalidFormats tests format validation for IDs.
func TestSpanValidation_InvalidFormats(t *testing.T) {
	tests := []struct {
		name        string
		span        Span
		expectedErr error
	}{
		{
			name: "trace_id too short",
			span: Span{
				TraceID:       "abc123",
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrInvalidTraceIDFormat,
		},
		{
			name: "trace_id non-hex",
			span: Span{
				TraceID:       "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrInvalidTraceIDFormat,
		},
		{
			name: "span_id too short",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        "abc",
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrInvalidSpanIDFormat,
		},
		{
			name: "span_id non-hex",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        "gggggggggggggggg",
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
			},
			expectedErr: ErrInvalidSpanIDFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.span.Validate()
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// TestSpanValidation_InvalidValues tests logical validation.
func TestSpanValidation_InvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		span        Span
		expectedErr error
	}{
		{
			name: "negative duration",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Duration:      -50 * time.Millisecond,
				Status:        "ok",
			},
			expectedErr: ErrNegativeDuration,
		},
		{
			name: "invalid status",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "maybe",
			},
			expectedErr: ErrInvalidStatus,
		},
		{
			name: "invalid span_kind",
			span: Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
				SpanKind:      "invalid",
			},
			expectedErr: ErrInvalidSpanKind,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.span.Validate()
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// TestSpanValidation_ValidSpanKinds verifies all valid span kinds are accepted.
func TestSpanValidation_ValidSpanKinds(t *testing.T) {
	validKinds := []string{"client", "server", "internal", "producer", "consumer"}

	for _, kind := range validKinds {
		t.Run("span_kind_"+kind, func(t *testing.T) {
			span := &Span{
				TraceID:       GenerateTraceID(),
				SpanID:        GenerateSpanID(),
				ServiceName:   "test",
				OperationName: "test",
				StartTime:     time.Now(),
				Status:        "ok",
				SpanKind:      kind,
			}

			if err := span.Validate(); err != nil {
				t.Errorf("valid span_kind '%s' failed validation: %v", kind, err)
			}
		})
	}
}

// TestSpanHelperMethods tests utility methods on Span.
func TestSpanHelperMethods(t *testing.T) {
	start := time.Now()
	duration := 100 * time.Millisecond

	span := &Span{
		TraceID:       GenerateTraceID(),
		SpanID:        GenerateSpanID(),
		ServiceName:   "test",
		OperationName: "test",
		StartTime:     start,
		Duration:      duration,
		Status:        "error",
		Tags:          map[string]string{"key": "value"},
	}

	// Test EndTime
	expectedEnd := start.Add(duration)
	if !span.EndTime().Equal(expectedEnd) {
		t.Errorf("EndTime() = %v, want %v", span.EndTime(), expectedEnd)
	}

	// Test IsError
	if !span.IsError() {
		t.Error("IsError() should return true for error status")
	}

	// Test GetTag
	if span.GetTag("key") != "value" {
		t.Errorf("GetTag('key') = %v, want 'value'", span.GetTag("key"))
	}
	if span.GetTag("missing") != "" {
		t.Errorf("GetTag('missing') = %v, want ''", span.GetTag("missing"))
	}

	// Test SetTag
	span.SetTag("newkey", "newvalue")
	if span.GetTag("newkey") != "newvalue" {
		t.Errorf("After SetTag, GetTag('newkey') = %v, want 'newvalue'", span.GetTag("newkey"))
	}

	// Test SetTag on nil map
	span2 := &Span{}
	span2.SetTag("key", "value")
	if span2.GetTag("key") != "value" {
		t.Error("SetTag should initialize Tags map if nil")
	}
}

// TestGenerateTraceID verifies trace ID properties.
func TestGenerateTraceID(t *testing.T) {
	id := GenerateTraceID()

	// Test length
	if len(id) != 32 {
		t.Errorf("GenerateTraceID() length = %d, want 32", len(id))
	}

	// Test non-empty
	if id == "" {
		t.Error("GenerateTraceID() returned empty string")
	}

	// Test valid hex
	if !isHex(id) {
		t.Errorf("GenerateTraceID() = %s is not valid hex", id)
	}

	// Test non-zero
	allZeros := true
	for i := 0; i < len(id); i++ {
		if id[i] != '0' {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("GenerateTraceID() returned all zeros")
	}
}

// TestGenerateSpanID verifies span ID properties.
func TestGenerateSpanID(t *testing.T) {
	id := GenerateSpanID()

	// Test length
	if len(id) != 16 {
		t.Errorf("GenerateSpanID() length = %d, want 16", len(id))
	}

	// Test non-empty
	if id == "" {
		t.Error("GenerateSpanID() returned empty string")
	}

	// Test valid hex
	if !isHex(id) {
		t.Errorf("GenerateSpanID() = %s is not valid hex", id)
	}

	// Test non-zero
	allZeros := true
	for i := 0; i < len(id); i++ {
		if id[i] != '0' {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("GenerateSpanID() returned all zeros")
	}
}

// TestConcurrentIDGeneration verifies that concurrent ID generation produces unique IDs.
// This is critical for a distributed tracing system where multiple goroutines
// generate IDs simultaneously.
func TestConcurrentIDGeneration(t *testing.T) {
	const goroutines = 100
	const idsPerGoroutine = 100

	t.Run("TraceIDs", func(t *testing.T) {
		ids := make(chan string, goroutines*idsPerGoroutine)
		var wg sync.WaitGroup

		// Generate IDs concurrently
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < idsPerGoroutine; j++ {
					ids <- GenerateTraceID()
				}
			}()
		}

		wg.Wait()
		close(ids)

		// Check for duplicates
		seen := make(map[string]bool)
		count := 0
		for id := range ids {
			if seen[id] {
				t.Fatalf("duplicate trace ID generated: %s", id)
			}
			seen[id] = true
			count++
		}

		if count != goroutines*idsPerGoroutine {
			t.Errorf("generated %d IDs, want %d", count, goroutines*idsPerGoroutine)
		}
	})

	t.Run("SpanIDs", func(t *testing.T) {
		ids := make(chan string, goroutines*idsPerGoroutine)
		var wg sync.WaitGroup

		// Generate IDs concurrently
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < idsPerGoroutine; j++ {
					ids <- GenerateSpanID()
				}
			}()
		}

		wg.Wait()
		close(ids)

		// Check for duplicates
		seen := make(map[string]bool)
		count := 0
		for id := range ids {
			if seen[id] {
				t.Fatalf("duplicate span ID generated: %s", id)
			}
			seen[id] = true
			count++
		}

		if count != goroutines*idsPerGoroutine {
			t.Errorf("generated %d IDs, want %d", count, goroutines*idsPerGoroutine)
		}
	})
}

// TestIsValidTraceID tests trace ID format validation.
func TestIsValidTraceID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid lowercase", "0123456789abcdef0123456789abcdef", true},
		{"valid uppercase", "0123456789ABCDEF0123456789ABCDEF", true},
		{"valid mixed case", "0123456789aBcDeF0123456789AbCdEf", true},
		{"too short", "0123456789abcdef", false},
		{"too long", "0123456789abcdef0123456789abcdef00", false},
		{"non-hex chars", "0123456789abcdefghij456789abcdef", false},
		{"empty string", "", false},
		{"special chars", "0123456789abcdef-123456789abcdef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTraceID(tt.id); got != tt.valid {
				t.Errorf("IsValidTraceID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

// TestIsValidSpanID tests span ID format validation.
func TestIsValidSpanID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid lowercase", "0123456789abcdef", true},
		{"valid uppercase", "0123456789ABCDEF", true},
		{"valid mixed case", "0123456789aBcDeF", true},
		{"too short", "0123456789ab", false},
		{"too long", "0123456789abcdef00", false},
		{"non-hex chars", "0123456789abcdez", false},
		{"empty string", "", false},
		{"special chars", "0123456789abcd-f", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidSpanID(tt.id); got != tt.valid {
				t.Errorf("IsValidSpanID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

// BenchmarkGenerateTraceID measures trace ID generation performance.
func BenchmarkGenerateTraceID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateTraceID()
	}
}

// BenchmarkGenerateSpanID measures span ID generation performance.
func BenchmarkGenerateSpanID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateSpanID()
	}
}

// BenchmarkSpanValidate measures validation performance.
func BenchmarkSpanValidate(b *testing.B) {
	span := &Span{
		TraceID:       GenerateTraceID(),
		SpanID:        GenerateSpanID(),
		ServiceName:   "test",
		OperationName: "test",
		StartTime:     time.Now(),
		Status:        "ok",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = span.Validate()
	}
}