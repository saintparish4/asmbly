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

