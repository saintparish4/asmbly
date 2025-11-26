# Benchmark Tests

## BenchmarkGenerateTraceID

### Overview
This benchmark measures the performance of trace ID generation, which is a critical operation in distributed tracing systems. Trace IDs must be generated frequently and efficiently as they are created for every new trace.

### Implementation
The benchmark tests the `GenerateTraceID()` function, which generates a cryptographically random 128-bit trace ID using `crypto/rand`.

```491:496:internal/models/trace_test.go
// BenchmarkGenerateTraceID measures trace ID generation performance.
func BenchmarkGenerateTraceID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateTraceID()
	}
}
```

### Function Under Test

```8:22:internal/models/ids.go
// GenerateTraceID creates a cryptographically random 128-bit trace ID.
// Returns a 32-character lowercase hex string (e.g., "a1b2c3d4e5f6...").
// 
// This uses crypto/rand for true randomness suitable for distributed systems,
// ensuring trace IDs are globally unique across all services.
func GenerateTraceID() string {
	b := make([]byte, 16) // 128 bits = 16 bytes
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand.Read only fails on catastrophic system errors
		// In practice, this should never happen on modern systems
		panic("failed to generate random trace ID: " + err.Error())
	}
	return hex.EncodeToString(b) // 16 bytes → 32 hex chars
}
```

### Results
```
BenchmarkGenerateTraceID-12      1717603               674.9 ns/op
```

### Performance Analysis
- **Iterations**: 1,717,603 operations
- **Time per operation**: 674.9 nanoseconds (approximately 0.675 microseconds)
- **Throughput**: ~1.48 million trace IDs per second
- **GOMAXPROCS**: 12 (indicates 12 parallel goroutines were used)

### Interpretation
The benchmark shows that generating a trace ID takes approximately **675 nanoseconds** per operation. This is excellent performance for a cryptographic random number generation operation, making it suitable for high-throughput distributed tracing systems where trace IDs are generated frequently.

The use of `crypto/rand` ensures cryptographically secure randomness, which is essential for distributed systems where trace IDs must be globally unique across all services and instances.

## BenchmarkEviction

### Overview
This benchmark measures the performance of the trace eviction mechanism in the in-memory storage system. Eviction is critical for preventing unbounded memory growth by removing old traces when the storage capacity is exceeded.

### Implementation
The benchmark tests the eviction behavior by pre-filling a store beyond its capacity and then measuring the cost of adding new traces that trigger the eviction process.

```366:400:internal/storage/memory_bench_test.go
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
```

### Functions Under Test

```457:473:internal/storage/memory.go
// maybeEvict checks if eviction is needed and evicts old traces if necessary.
func (s *MemoryStore) maybeEvict() {
	// Count traces
	var count int
	s.traces.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count <= s.maxTraces {
		return
	}

	// Simple eviction: remove oldest traces
	// In production, this would be LRU with timestamps
	s.evictOldTraces(count - s.maxTraces)
}
```

### Results
```
BenchmarkEviction-16             37774             31217 ns/op           24617 B/op         39 allocs/op
```

### Performance Analysis
- **Iterations**: 37,774 operations
- **Time per operation**: 31,217 nanoseconds (approximately 31.2 microseconds)
- **Memory per operation**: 24,617 bytes allocated
- **Allocations per operation**: 39 allocations
- **GOMAXPROCS**: 16 (indicates 16 parallel goroutines were used)
- **Throughput**: ~32,000 eviction-triggering writes per second

### Interpretation
The benchmark shows that writing a span that triggers eviction takes approximately **31.2 microseconds** per operation. This includes:
1. Span validation and storage
2. Index updates (service, timestamp, duration, and cost indexes)
3. Trace counting and eviction check
4. Eviction of oldest traces (sorting, span cleanup, index cleanup)

The eviction mechanism uses a time-based approach where it:
- Counts all traces using `sync.Map.Range`
- Collects trace metadata including timestamps
- Sorts traces by start time to identify oldest traces
- Removes spans, traces, and updates all indexes

With 39 allocations per operation, the eviction process involves significant memory management, which is expected given the need to collect trace metadata, sort, and update multiple data structures.

### Key Insights
- **Acceptable overhead**: 31.2μs is reasonable for a write operation that includes eviction, especially compared to typical network latencies (milliseconds)
- **Memory management**: The 24KB allocation per operation includes trace ID/span ID generation, span structure, and temporary collections during eviction
- **Optimization opportunity**: The current implementation counts traces on every write; future optimizations could maintain a `traceCount` field to avoid the `Range` operation
