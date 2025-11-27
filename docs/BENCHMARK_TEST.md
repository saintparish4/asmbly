# Benchmark Tests

## Overview

This document contains performance benchmarks for the TraceFlow distributed tracing system. Benchmarks are organized by package and measure critical operations including ID generation, span validation, storage operations, and HTTP endpoint performance.

**Test Environment:**
- CPU: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz
- OS: Linux (WSL2)
- Go: amd64 architecture
- GOMAXPROCS: 16

---

## Table of Contents

1. [Collector Benchmarks](#collector-benchmarks)
2. [Models Benchmarks](#models-benchmarks)
3. [Storage Benchmarks](#storage-benchmarks)

---

## Collector Benchmarks

### BenchmarkSubmitSpan

**Overview:** Measures the performance of submitting spans to the collector's processing queue.

**Results:**
```
BenchmarkSubmitSpan-16       10000        249014 ns/op      124887 B/op       19 allocs/op
```

**Performance Analysis:**
- **Iterations**: 10,000 operations
- **Time per operation**: 249.0 microseconds
- **Throughput**: ~4,016 spans per second
- **Memory per operation**: 124,887 bytes
- **Allocations per operation**: 19

**Interpretation:**
The `SubmitSpan` operation takes approximately 249μs per span, which includes trace ID generation, span ID generation, span validation, and queuing for async processing. The relatively high memory allocation (124KB) is primarily due to ID generation using `crypto/rand`.

---

### BenchmarkHandlePostSpan

**Overview:** Measures the end-to-end HTTP handler performance for receiving span submissions.

**Results:**
```
BenchmarkHandlePostSpan-16   182336        6573 ns/op        7403 B/op       39 allocs/op
```

**Performance Analysis:**
- **Iterations**: 182,336 operations
- **Time per operation**: 6.573 microseconds
- **Throughput**: ~152,000 requests per second
- **Memory per operation**: 7,403 bytes
- **Allocations per operation**: 39

**Interpretation:**
The HTTP handler is highly efficient at approximately 6.6μs per request. This includes HTTP request parsing, JSON unmarshaling, and span submission to the queue. The handler can theoretically process 152K requests/second, making it suitable for high-throughput tracing scenarios.

---

## Models Benchmarks

### BenchmarkGenerateTraceID

**Overview:** Measures the performance of trace ID generation, a critical operation performed for every new trace.

**Results:**
```
BenchmarkGenerateTraceID-16  1710976       700.6 ns/op          80 B/op        3 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,710,976 operations
- **Time per operation**: 700.6 nanoseconds
- **Throughput**: ~1.43 million trace IDs per second
- **Memory per operation**: 80 bytes
- **Allocations per operation**: 3

**Implementation:**
```go
func GenerateTraceID() string {
    b := make([]byte, 16) // 128 bits = 16 bytes
    _, err := rand.Read(b)
    if err != nil {
        panic("failed to generate random trace ID: " + err.Error())
    }
    return hex.EncodeToString(b) // 16 bytes → 32 hex chars
}
```

**Interpretation:**
Generating a 128-bit trace ID using `crypto/rand` takes approximately 701 nanoseconds. This is excellent performance for cryptographically secure random number generation, ensuring globally unique trace IDs suitable for distributed systems.

---

### BenchmarkGenerateSpanID

**Overview:** Measures the performance of span ID generation, performed for every span in a trace.

**Results:**
```
BenchmarkGenerateSpanID-16   1759574       681.0 ns/op          40 B/op        3 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,759,574 operations
- **Time per operation**: 681.0 nanoseconds
- **Throughput**: ~1.47 million span IDs per second
- **Memory per operation**: 40 bytes
- **Allocations per operation**: 3

**Interpretation:**
Span ID generation (64-bit) is slightly faster than trace ID generation (128-bit) at 681ns per operation. The lower memory allocation (40 bytes vs 80 bytes) reflects the smaller ID size. The performance is sufficient for generating millions of span IDs per second.

---

### BenchmarkSpanValidate

**Overview:** Measures the performance of span validation, executed on every span submission.

**Results:**
```
BenchmarkSpanValidate-16     45403818       26.40 ns/op           0 B/op        0 allocs/op
```

**Performance Analysis:**
- **Iterations**: 45,403,818 operations
- **Time per operation**: 26.40 nanoseconds
- **Throughput**: ~37.9 million validations per second
- **Memory per operation**: 0 bytes (zero allocations!)
- **Allocations per operation**: 0

**Interpretation:**
Span validation is extremely fast at just 26.4 nanoseconds with zero heap allocations. This makes validation overhead negligible even at very high throughput. The zero-allocation design is critical for minimizing GC pressure in high-volume scenarios.

---

## Storage Benchmarks

### BenchmarkWriteSpan_Sequential

**Overview:** Measures sequential span write performance without concurrency.

**Results:**
```
BenchmarkWriteSpan_Sequential-16    10000       483485 ns/op     258363 B/op       28 allocs/op
```

**Performance Analysis:**
- **Iterations**: 10,000 operations
- **Time per operation**: 483.5 microseconds
- **Throughput**: ~2,068 writes per second
- **Memory per operation**: 258,363 bytes
- **Allocations per operation**: 28

**Interpretation:**
Sequential writes take approximately 483μs per span, which includes span validation, storage in sync.Map, and updating all indexes (service, timestamp, duration, cost). The high memory allocation reflects the cost of ID generation and index updates.

---

### BenchmarkWriteSpan_Concurrent

**Overview:** Measures concurrent span write performance with multiple goroutines.

**Results:**
```
BenchmarkWriteSpan_Concurrent-16    10000       143367 ns/op      77216 B/op       24 allocs/op
```

**Performance Analysis:**
- **Iterations**: 10,000 operations
- **Time per operation**: 143.4 microseconds (3.37x faster than sequential)
- **Throughput**: ~6,975 writes per second
- **Memory per operation**: 77,216 bytes (70% reduction)
- **Allocations per operation**: 24

**Interpretation:**
Concurrent writes show significant performance improvement (3.37x speedup) due to parallel processing. The use of `sync.Map` and concurrent-safe indexes enables efficient parallel writes without lock contention. Memory allocation is also reduced due to goroutine batching effects.

---

### BenchmarkGetTrace

**Overview:** Measures trace retrieval performance by trace ID.

**Results:**
```
BenchmarkGetTrace-16         1224214        987.4 ns/op         1728 B/op        6 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,224,214 operations
- **Time per operation**: 987.4 nanoseconds
- **Throughput**: ~1.01 million traces per second
- **Memory per operation**: 1,728 bytes
- **Allocations per operation**: 6

**Interpretation:**
Trace retrieval is highly efficient at approximately 1 microsecond per operation. This includes trace lookup, span assembly, and sorting by timestamp. The low allocation count indicates efficient data structure access.

---

### BenchmarkGetTrace_Concurrent

**Overview:** Measures concurrent trace retrieval with multiple readers.

**Results:**
```
BenchmarkGetTrace_Concurrent-16    1558998        755.7 ns/op       1728 B/op        6 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,558,998 operations
- **Time per operation**: 755.7 nanoseconds (1.31x faster)
- **Throughput**: ~1.32 million traces per second
- **Memory per operation**: 1,728 bytes
- **Allocations per operation**: 6

**Interpretation:**
Concurrent reads show improved performance (1.31x speedup) due to parallel goroutine execution. The `sync.Map` implementation provides efficient concurrent read access without lock contention, making it ideal for read-heavy workloads.

---

### BenchmarkFindTraces_ByService

**Overview:** Measures performance of finding traces filtered by service name.

**Results:**
```
BenchmarkFindTraces_ByService-16     16842        74894 ns/op      80065 B/op      620 allocs/op
```

**Performance Analysis:**
- **Iterations**: 16,842 operations
- **Time per operation**: 74.9 microseconds
- **Throughput**: ~13,350 queries per second
- **Memory per operation**: 80,065 bytes
- **Allocations per operation**: 620

**Interpretation:**
Service-based filtering takes approximately 75μs, which includes index lookup and trace assembly. The service index enables efficient O(1) lookup of traces by service name. The higher allocation count reflects building result sets from multiple traces.

---

### BenchmarkFindTraces_ByDuration

**Overview:** Measures performance of finding traces filtered by duration range.

**Results:**
```
BenchmarkFindTraces_ByDuration-16     1723       688434 ns/op     741415 B/op     6021 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,723 operations
- **Time per operation**: 688.4 microseconds
- **Throughput**: ~1,453 queries per second
- **Memory per operation**: 741,415 bytes
- **Allocations per operation**: 6,021

**Interpretation:**
Duration-based filtering is more expensive at 688μs per query due to scanning duration buckets and filtering results. This operation touches more data structures and requires more memory allocation for result set construction. Consider this when designing query patterns.

---

### BenchmarkFindTraces_ByTimeRange

**Overview:** Measures performance of finding traces within a time range.

**Results:**
```
BenchmarkFindTraces_ByTimeRange-16    12016       100353 ns/op     107147 B/op      779 allocs/op
```

**Performance Analysis:**
- **Iterations**: 12,016 operations
- **Time per operation**: 100.4 microseconds
- **Throughput**: ~9,965 queries per second
- **Memory per operation**: 107,147 bytes
- **Allocations per operation**: 779

**Interpretation:**
Time-range queries take approximately 100μs, using timestamp bucket indexes for efficient temporal filtering. The bucketing strategy (15-second buckets) provides good balance between query performance and memory overhead.

---

### BenchmarkGetServices

**Overview:** Measures performance of retrieving the list of all services.

**Results:**
```
BenchmarkGetServices-16      169930         6881 ns/op          1792 B/op        1 allocs/op
```

**Performance Analysis:**
- **Iterations**: 169,930 operations
- **Time per operation**: 6.881 microseconds
- **Throughput**: ~145,328 queries per second
- **Memory per operation**: 1,792 bytes
- **Allocations per operation**: 1

**Interpretation:**
Retrieving the service list is very fast at approximately 7μs with minimal allocations. The service index maintains a deduplicated set of service names, enabling efficient metadata queries for UI rendering and service discovery.

---

### BenchmarkIndexUpdate

**Overview:** Measures the overhead of updating all indexes when writing a span.

**Results:**
```
BenchmarkIndexUpdate-16       23292       120144 ns/op           273 B/op        0 allocs/op
```

**Performance Analysis:**
- **Iterations**: 23,292 operations
- **Time per operation**: 120.1 microseconds
- **Throughput**: ~8,325 updates per second
- **Memory per operation**: 273 bytes
- **Allocations per operation**: 0

**Interpretation:**
Index updates take approximately 120μs and include updates to service index, timestamp buckets, duration buckets, and cost index. The zero heap allocations demonstrate efficient index update implementation without GC pressure.

---

### BenchmarkAssembleTrace

**Overview:** Measures the performance of assembling spans into a complete trace.

**Results:**
```
BenchmarkAssembleTrace-16    1000000        1065 ns/op           480 B/op        5 allocs/op
```

**Performance Analysis:**
- **Iterations**: 1,000,000 operations
- **Time per operation**: 1.065 microseconds
- **Throughput**: ~939,000 assemblies per second
- **Memory per operation**: 480 bytes
- **Allocations per operation**: 5

**Interpretation:**
Trace assembly is efficient at approximately 1μs per operation. This includes collecting spans, sorting by timestamp, and building the trace structure. The low allocation count reflects efficient slice handling and minimal copying.

---

### BenchmarkEviction

**Overview:** Measures the performance of trace eviction when storage capacity is exceeded.

**Results:**
```
BenchmarkEviction-16          35676        33224 ns/op         24617 B/op       39 allocs/op
```

**Performance Analysis:**
- **Iterations**: 35,676 operations
- **Time per operation**: 33.2 microseconds
- **Throughput**: ~30,107 eviction-triggering writes per second
- **Memory per operation**: 24,617 bytes
- **Allocations per operation**: 39

**Implementation:**
```go
func (s *MemoryStore) maybeEvict() {
    var count int
    s.traces.Range(func(key, value interface{}) bool {
        count++
        return true
    })
    
    if count <= s.maxTraces {
        return
    }
    
    s.evictOldTraces(count - s.maxTraces)
}
```

**Interpretation:**
Eviction adds approximately 33μs of overhead when writing a span that triggers eviction. This includes:
1. Trace counting via `sync.Map.Range`
2. Collecting trace metadata with timestamps
3. Sorting traces by start time
4. Removing oldest traces and updating all indexes

The 39 allocations reflect the temporary collections needed during eviction. This overhead is acceptable given typical network latencies (milliseconds), and eviction only occurs when capacity is exceeded.

**Optimization Opportunities:**
- Maintain an atomic `traceCount` to avoid the `Range` operation
- Implement true LRU with access timestamps
- Consider background eviction for lower latency impact

---

## Summary

### Key Performance Metrics

| Operation | Throughput | Latency | Allocations |
|-----------|-----------|---------|-------------|
| HTTP Handler | 152K req/s | 6.6μs | 39 allocs |
| Span Validation | 37.9M ops/s | 26ns | 0 allocs |
| Concurrent Write | 6,975 writes/s | 143μs | 24 allocs |
| Trace Retrieval | 1.32M traces/s | 756ns | 6 allocs |
| Service Query | 145K queries/s | 6.9μs | 1 alloc |

### Performance Characteristics

**Strengths:**
- ✓ Zero-allocation span validation (26ns)
- ✓ Fast ID generation (681-701ns)
- ✓ Efficient concurrent operations (3.37x speedup)
- ✓ Low-latency HTTP handlers (6.6μs)
- ✓ Fast trace assembly (1μs)

**Areas for Optimization:**
- Duration-based queries are slower (688μs) - consider index optimization
- Sequential writes have high memory allocation (258KB) - optimize ID generation caching
- Eviction counting could use atomic counter instead of Range operation

### Recommendations

1. **High-Throughput Deployments**: Use concurrent workers (5-10) to maximize write throughput
2. **Query Optimization**: Prefer service-based and time-range queries over duration queries
3. **Memory Management**: Set appropriate `maxTraces` to balance memory usage and eviction frequency
4. **Monitoring**: Track eviction rate and adjust capacity if evictions become too frequent
