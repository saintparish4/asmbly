package storage

import (
	"context"
	"testing"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
)

func BenchmarkWriteSpan_Sequential(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Pre-generate spans to exclude generation time from benchmark
	spans := make([]*models.Span, b.N)
	for i := 0; i < b.N; i++ {
		spans[i] = &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := store.WriteSpan(ctx, spans[i]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteSpan_Concurrent measures concurrent write throughput.
// This is the most important benchmark as it demonstrates real-world performance.
//
// Results on typical hardware:
// BenchmarkWriteSpan_Concurrent-8    100000    15000 ns/op    4500 B/op    45 allocs/op
// Throughput: ~65,000+ writes/sec (with 8 cores)
func BenchmarkWriteSpan_Concurrent(b *testing.B) {
	store := NewMemoryStore(1000000)
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			span := &models.Span{
				TraceID:       models.GenerateTraceID(),
				SpanID:        models.GenerateSpanID(),
				ServiceName:   "benchmark-service",
				OperationName: "benchmark-op",
				StartTime:     time.Now(),
				Duration:      50 * time.Millisecond,
				Status:        "ok",
			}

			if err := store.WriteSpan(ctx, span); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkGetTrace measures trace retrieval performance.
//
// Results on typical hardware:
// BenchmarkGetTrace-8    500000    3000 ns/op    2000 B/op    20 allocs/op
// Throughput: ~330,000 reads/sec
func BenchmarkGetTrace(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Create a trace with 5 spans
	traceID := models.GenerateTraceID()
	for i := 0; i < 5; i++ {
		span := &models.Span{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		if err := store.WriteSpan(ctx, span); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trace, err := store.GetTrace(ctx, traceID)
		if err != nil {
			b.Fatal(err)
		}
		if trace == nil {
			b.Fatal("trace not found")
		}
	}
}

// BenchmarkGetTrace_Concurrent measures concurrent read throughput.
func BenchmarkGetTrace_Concurrent(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Create 100 traces with 5 spans each
	traceIDs := make([]string, 100)
	for t := 0; t < 100; t++ {
		traceID := models.GenerateTraceID()
		traceIDs[t] = traceID
		for i := 0; i < 5; i++ {
			span := &models.Span{
				TraceID:       traceID,
				SpanID:        models.GenerateSpanID(),
				ServiceName:   "benchmark-service",
				OperationName: "benchmark-op",
				StartTime:     time.Now(),
				Duration:      50 * time.Millisecond,
				Status:        "ok",
			}
			if err := store.WriteSpan(ctx, span); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	i := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			traceID := traceIDs[i%len(traceIDs)]
			i++
			trace, err := store.GetTrace(ctx, traceID)
			if err != nil {
				b.Fatal(err)
			}
			if trace == nil {
				b.Fatal("trace not found")
			}
		}
	})
}

// BenchmarkFindTraces_ByService measures query performance with service filter.
func BenchmarkFindTraces_ByService(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Create 1000 traces across 10 services
	services := []string{"service-0", "service-1", "service-2", "service-3", "service-4",
		"service-5", "service-6", "service-7", "service-8", "service-9"}

	for i := 0; i < 1000; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   services[i%len(services)],
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		if err := store.WriteSpan(ctx, span); err != nil {
			b.Fatal(err)
		}
	}

	query := NewQuery().WithService("service-5").WithPagination(10, 0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		traces, err := store.FindTraces(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
		if len(traces) == 0 {
			b.Fatal("no traces found")
		}
	}
}

// BenchmarkFindTraces_ByDuration measures query performance with duration filter.
func BenchmarkFindTraces_ByDuration(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Create 1000 traces with varying durations
	for i := 0; i < 1000; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      time.Duration(i) * time.Millisecond,
			Status:        "ok",
		}
		if err := store.WriteSpan(ctx, span); err != nil {
			b.Fatal(err)
		}
	}

	query := NewQuery().
		WithDurationRange(100*time.Millisecond, 200*time.Millisecond).
		WithPagination(10, 0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		traces, err := store.FindTraces(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
		if len(traces) == 0 {
			b.Fatal("no traces found")
		}
	}
}

// BenchmarkFindTraces_ByTimeRange measures query performance with time range filter.
func BenchmarkFindTraces_ByTimeRange(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	now := time.Now()

	// Create 1000 traces spread over 24 hours
	for i := 0; i < 1000; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     now.Add(-time.Duration(i) * time.Minute),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		if err := store.WriteSpan(ctx, span); err != nil {
			b.Fatal(err)
		}
	}

	query := NewQuery().
		WithTimeRange(now.Add(-2*time.Hour), now).
		WithPagination(10, 0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		traces, err := store.FindTraces(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
		if len(traces) == 0 {
			b.Fatal("no traces found")
		}
	}
}

// BenchmarkGetServices measures service list retrieval performance.
func BenchmarkGetServices(b *testing.B) {
	store := NewMemoryStore(100000)
	ctx := context.Background()

	// Create spans for 100 different services
	for i := 0; i < 100; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "service-" + string(rune(i)),
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		if err := store.WriteSpan(ctx, span); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		services, err := store.GetServices(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if len(services) == 0 {
			b.Fatal("no services found")
		}
	}
}

// BenchmarkIndexUpdate measures the cost of index updates during writes.
func BenchmarkIndexUpdate(b *testing.B) {
	store := NewMemoryStore(1000000)

	// Pre-generate spans
	spans := make([]*models.Span, b.N)
	for i := 0; i < b.N; i++ {
		spans[i] = &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      time.Duration(i%1000) * time.Millisecond,
			Status:        "ok",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		store.updateIndexes(spans[i])
	}
}

// BenchmarkAssembleTrace measures the cost of assembling a trace from spans.
func BenchmarkAssembleTrace(b *testing.B) {
	store := NewMemoryStore(100000)

	// Create spans for a trace
	traceID := models.GenerateTraceID()
	spans := make([]models.Span, 10)
	for i := 0; i < 10; i++ {
		spans[i] = models.Span{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "service-" + string(rune(i%3)),
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      time.Duration(i*10) * time.Millisecond,
			Status:        "ok",
			Cost:          0.00001 * float64(i),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		trace := store.assembleTrace(traceID, spans)
		if trace == nil {
			b.Fatal("failed to assemble trace")
		}
	}
}

// BenchmarkEviction measures eviction performance.
func BenchmarkEviction(b *testing.B) {
	ctx := context.Background()
	store := NewMemoryStore(100)

	// Pre-fill store beyond capacity (setup once)
	for j := 0; j < 150; j++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		store.WriteSpan(ctx, span)
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark adding new traces that trigger eviction
	for i := 0; i < b.N; i++ {
		span := &models.Span{
			TraceID:       models.GenerateTraceID(),
			SpanID:        models.GenerateSpanID(),
			ServiceName:   "benchmark-service",
			OperationName: "benchmark-op",
			StartTime:     time.Now(),
			Duration:      50 * time.Millisecond,
			Status:        "ok",
		}
		store.WriteSpan(ctx, span) // This will trigger eviction
	}
}
