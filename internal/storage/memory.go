package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
)

// MemoryStore is a concurrent-safe in-memory trace storage implementation
// It uses sync.Map for lock-free reads and maintains multiple indexes for efficiency
type MemoryStore struct {
	// Core storage - concurrent-safe maps
	spans  sync.Map // spanID (string) -> *models.Span
	traces sync.Map // traceID (string) -> []string (spanIDs)

	// Indexes for efficient queries
	indexes *Indexes
	indexMu sync.RWMutex // protects indexes updates

	// Config
	maxTraces int // Max traces to keep in memory

	// Metrics
	spanCount  int64
	traceCount int64
	mu         sync.RWMutex // Protects counters
}

// Indexes maintains multiple indexes for efficient trace queries.
type Indexes struct {
	// Service index: service name → []traceID
	byService map[string][]string

	// Time buckets: hourly buckets for temporal queries
	byTimestamp *TimeBuckets

	// Duration buckets: categorize traces by duration
	byDuration *DurationBuckets

	// Cost buckets: categorize traces by cost (Week 3)
	byCost *CostBuckets
}

// TimeBuckets organizes traces by hourly time buckets for efficient time-range queries.
type TimeBuckets struct {
	buckets map[int64][]string // Unix hour → []traceID
}

// DurationBuckets categorizes traces by duration for efficient duration queries.
type DurationBuckets struct {
	fast     []string // < 10ms
	medium   []string // 10ms - 100ms
	slow     []string // 100ms - 1s
	verySlow []string // > 1s
}

// CostBuckets categorizes traces by cost for efficient cost queries.
type CostBuckets struct {
	cheap     []string // < $0.0001
	moderate  []string // $0.0001 - $0.001
	expensive []string // > $0.001
}

// NewMemoryStore creates a new in-memory storage with the given capacity.
// maxTraces controls how many traces to keep before evicting old ones.
func NewMemoryStore(maxTraces int) *MemoryStore {
	return &MemoryStore{
		maxTraces: maxTraces,
		indexes: &Indexes{
			byService:   make(map[string][]string),
			byTimestamp: &TimeBuckets{buckets: make(map[int64][]string)},
			byDuration:  &DurationBuckets{},
			byCost:      &CostBuckets{},
		},
	}
}

// WriteSpan stores a span and updates all indexes.
// This method is safe for concurrent use.
func (s *MemoryStore) WriteSpan(ctx context.Context, span *models.Span) error {
	// Validate span before storing
	if err := span.Validate(); err != nil {
		return fmt.Errorf("invalid span: %w", err)
	}

	// Store span in main map
	s.spans.Store(span.SpanID, span)

	// Add span to trace's span list
	s.addSpanToTrace(span.TraceID, span.SpanID)

	// Update indexes
	s.updateIndexes(span)

	// Update counters
	s.mu.Lock()
	s.spanCount++
	s.mu.Unlock()

	// Check if eviction is needed
	s.maybeEvict()

	return nil
}

// GetTrace retrieves and assembles a complete trace by ID.
func (s *MemoryStore) GetTrace(ctx context.Context, traceID string) (*models.Trace, error) {
	// Get span IDs for this trace
	value, ok := s.traces.Load(traceID)
	if !ok {
		return nil, nil // Trace not found
	}

	spanIDs := value.([]string)
	if len(spanIDs) == 0 {
		return nil, nil
	}

	// Retrieve all spans
	spans := make([]models.Span, 0, len(spanIDs))
	for _, spanID := range spanIDs {
		if value, ok := s.spans.Load(spanID); ok {
			span := value.(*models.Span)
			spans = append(spans, *span)
		}
	}

	if len(spans) == 0 {
		return nil, nil
	}

	// Assemble trace metadata
	trace := s.assembleTrace(traceID, spans)
	return trace, nil
}

// FindTraces searches for traces matching the query criteria.
func (s *MemoryStore) FindTraces(ctx context.Context, query *Query) ([]*models.Trace, error) {
	// Get candidate trace IDs from indexes
	candidates := s.getCandidateTraces(query)

	// Filter candidates and build results
	var results []*models.Trace
	for _, traceID := range candidates {
		trace, err := s.GetTrace(ctx, traceID)
		if err != nil {
			continue
		}
		if trace == nil {
			continue
		}

		// Apply filters
		if s.matchesQuery(trace, query) {
			results = append(results, trace)
		}
	}

	// Sort by start time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.After(results[j].StartTime)
	})

	// Apply pagination
	total := len(results)
	if query.Offset >= total {
		return []*models.Trace{}, nil
	}

	end := query.Offset + query.Limit
	if query.Limit == 0 {
		end = total
	} else if end > total {
		end = total
	}

	return results[query.Offset:end], nil
}

// GetServices returns all unique service names.
func (s *MemoryStore) GetServices(ctx context.Context) ([]string, error) {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()

	services := make([]string, 0, len(s.indexes.byService))
	for service := range s.indexes.byService {
		services = append(services, service)
	}

	sort.Strings(services)
	return services, nil
}

// Close cleanly shuts down the storage (no-op for in-memory).
func (s *MemoryStore) Close() error {
	return nil
}

// addSpanToTrace adds a span ID to a trace's span list.
func (s *MemoryStore) addSpanToTrace(traceID, spanID string) {
	// Load existing span IDs or create new slice
	value, loaded := s.traces.LoadOrStore(traceID, []string{})
	spanIDs := value.([]string)

	// If this is a new trace, increment counter
	if !loaded {
		s.mu.Lock()
		s.traceCount++
		s.mu.Unlock()
	}

	// Check if span already exists (idempotency)
	for _, id := range spanIDs {
		if id == spanID {
			return
		}
	}

	// Add new span ID
	spanIDs = append(spanIDs, spanID)
	s.traces.Store(traceID, spanIDs)
}

// updateIndexes updates all indexes with the new span's information.
func (s *MemoryStore) updateIndexes(span *models.Span) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	// Index by service name
	if !s.containsString(s.indexes.byService[span.ServiceName], span.TraceID) {
		s.indexes.byService[span.ServiceName] = append(
			s.indexes.byService[span.ServiceName],
			span.TraceID,
		)
	}

	// Index by timestamp (hourly buckets)
	hourBucket := span.StartTime.Unix() / 3600
	if !s.containsString(s.indexes.byTimestamp.buckets[hourBucket], span.TraceID) {
		s.indexes.byTimestamp.buckets[hourBucket] = append(
			s.indexes.byTimestamp.buckets[hourBucket],
			span.TraceID,
		)
	}

	// Note: Duration and cost indexes are updated when trace is complete
	// For now, we'll index on first span (root span typically)
	if span.ParentSpanID == "" {
		// This is likely a root span
		s.updateDurationIndex(span.TraceID, span.Duration)
		s.updateCostIndex(span.TraceID, span.Cost)
	}
}

// updateDurationIndex categorizes a trace by duration.
func (s *MemoryStore) updateDurationIndex(traceID string, duration time.Duration) {
	ms := duration.Milliseconds()

	switch {
	case ms < 10:
		if !s.containsString(s.indexes.byDuration.fast, traceID) {
			s.indexes.byDuration.fast = append(s.indexes.byDuration.fast, traceID)
		}
	case ms < 100:
		if !s.containsString(s.indexes.byDuration.medium, traceID) {
			s.indexes.byDuration.medium = append(s.indexes.byDuration.medium, traceID)
		}
	case ms < 1000:
		if !s.containsString(s.indexes.byDuration.slow, traceID) {
			s.indexes.byDuration.slow = append(s.indexes.byDuration.slow, traceID)
		}
	default:
		if !s.containsString(s.indexes.byDuration.verySlow, traceID) {
			s.indexes.byDuration.verySlow = append(s.indexes.byDuration.verySlow, traceID)
		}
	}
}

// updateCostIndex categorizes a trace by cost.
func (s *MemoryStore) updateCostIndex(traceID string, cost float64) {
	switch {
	case cost < 0.0001:
		if !s.containsString(s.indexes.byCost.cheap, traceID) {
			s.indexes.byCost.cheap = append(s.indexes.byCost.cheap, traceID)
		}
	case cost < 0.001:
		if !s.containsString(s.indexes.byCost.moderate, traceID) {
			s.indexes.byCost.moderate = append(s.indexes.byCost.moderate, traceID)
		}
	default:
		if !s.containsString(s.indexes.byCost.expensive, traceID) {
			s.indexes.byCost.expensive = append(s.indexes.byCost.expensive, traceID)
		}
	}
}

// getCandidateTraces uses indexes to get a set of candidate trace IDs.
func (s *MemoryStore) getCandidateTraces(query *Query) []string {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()

	var candidates []string

	// Use service index if service filter is specified
	if query.Service != "" {
		candidates = s.indexes.byService[query.Service]
		return s.deduplicate(candidates)
	}

	// Use time index if time range is specified
	if !query.StartTime.IsZero() || !query.EndTime.IsZero() {
		candidates = s.getTracesInTimeRange(query.StartTime, query.EndTime)
		return s.deduplicate(candidates)
	}

	// Otherwise, get all traces
	s.traces.Range(func(key, value interface{}) bool {
		traceID := key.(string)
		candidates = append(candidates, traceID)
		return true
	})

	return candidates
}

// getTracesInTimeRange retrieves trace IDs within a time range using hourly buckets.
func (s *MemoryStore) getTracesInTimeRange(start, end time.Time) []string {
	if start.IsZero() {
		start = time.Unix(0, 0)
	}
	if end.IsZero() {
		end = time.Now().Add(24 * time.Hour)
	}

	var traceIDs []string

	startHour := start.Unix() / 3600
	endHour := end.Unix() / 3600

	for hour := startHour; hour <= endHour; hour++ {
		if bucket, ok := s.indexes.byTimestamp.buckets[hour]; ok {
			traceIDs = append(traceIDs, bucket...)
		}
	}

	return traceIDs
}

// matchesQuery checks if a trace matches all query filters.
func (s *MemoryStore) matchesQuery(trace *models.Trace, query *Query) bool {
	// Service filter
	if query.Service != "" {
		found := false
		for _, service := range trace.Services {
			if service == query.Service {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Duration filters
	if query.MinDuration > 0 && trace.Duration < query.MinDuration {
		return false
	}
	if query.MaxDuration > 0 && trace.Duration > query.MaxDuration {
		return false
	}

	// Cost filters (Week 3 feature)
	if query.MinCost > 0 && trace.TotalCost < query.MinCost {
		return false
	}
	if query.MaxCost > 0 && trace.TotalCost > query.MaxCost {
		return false
	}

	// Time range filters
	if !query.StartTime.IsZero() && trace.StartTime.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && trace.StartTime.After(query.EndTime) {
		return false
	}

	return true
}

// assembleTrace constructs a Trace from a collection of spans.
func (s *MemoryStore) assembleTrace(traceID string, spans []models.Span) *models.Trace {
	if len(spans) == 0 {
		return nil
	}

	// Find earliest start time and calculate total duration
	var startTime time.Time
	var endTime time.Time

	for i, span := range spans {
		if i == 0 || span.StartTime.Before(startTime) {
			startTime = span.StartTime
		}
		spanEnd := span.EndTime()
		if i == 0 || spanEnd.After(endTime) {
			endTime = spanEnd
		}
	}

	duration := endTime.Sub(startTime)

	// Collect unique services
	serviceSet := make(map[string]bool)
	for _, span := range spans {
		serviceSet[span.ServiceName] = true
	}
	services := make([]string, 0, len(serviceSet))
	for service := range serviceSet {
		services = append(services, service)
	}
	sort.Strings(services)

	// Calculate total cost (sum of all span costs)
	var totalCost float64
	costBreakdown := make(map[string]float64)
	for _, span := range spans {
		totalCost += span.Cost
		costBreakdown[span.ServiceName] += span.Cost
	}

	// Collect deployment info
	deployments := make(map[string]string)
	for _, span := range spans {
		if span.DeploymentID != "" {
			deployments[span.ServiceName] = span.DeploymentID
		}
	}

	return &models.Trace{
		TraceID:       traceID,
		Spans:         spans,
		StartTime:     startTime,
		Duration:      duration,
		Services:      services,
		Deployments:   deployments,
		TotalCost:     totalCost,
		CostBreakdown: costBreakdown,
	}
}

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

// evictOldTraces removes the oldest n traces.
func (s *MemoryStore) evictOldTraces(n int) {
	// Collect all traces with timestamps
	type traceInfo struct {
		traceID   string
		startTime time.Time
	}

	var traces []traceInfo
	s.traces.Range(func(key, value interface{}) bool {
		traceID := key.(string)
		spanIDs := value.([]string)
		if len(spanIDs) > 0 {
			if value, ok := s.spans.Load(spanIDs[0]); ok {
				span := value.(*models.Span)
				traces = append(traces, traceInfo{
					traceID:   traceID,
					startTime: span.StartTime,
				})
			}
		}
		return true
	})

	// Sort by start time (oldest first)
	sort.Slice(traces, func(i, j int) bool {
		return traces[i].startTime.Before(traces[j].startTime)
	})

	// Evict oldest n traces
	for i := 0; i < n && i < len(traces); i++ {
		s.evictTrace(traces[i].traceID)
	}
}

// evictTrace removes a trace and all its spans from storage and indexes.
func (s *MemoryStore) evictTrace(traceID string) {
	// Get span IDs
	value, ok := s.traces.Load(traceID)
	if !ok {
		return
	}

	spanIDs := value.([]string)

	// Delete all spans
	for _, spanID := range spanIDs {
		s.spans.Delete(spanID)
	}

	// Delete trace
	s.traces.Delete(traceID)

	// Decrement trace counter
	s.mu.Lock()
	s.traceCount--
	s.mu.Unlock()

	// Clean up indexes (simplified - in production, would track references)
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	// Remove from all indexes
	for service := range s.indexes.byService {
		s.indexes.byService[service] = s.removeString(s.indexes.byService[service], traceID)
	}

	for hour := range s.indexes.byTimestamp.buckets {
		s.indexes.byTimestamp.buckets[hour] = s.removeString(s.indexes.byTimestamp.buckets[hour], traceID)
	}

	s.indexes.byDuration.fast = s.removeString(s.indexes.byDuration.fast, traceID)
	s.indexes.byDuration.medium = s.removeString(s.indexes.byDuration.medium, traceID)
	s.indexes.byDuration.slow = s.removeString(s.indexes.byDuration.slow, traceID)
	s.indexes.byDuration.verySlow = s.removeString(s.indexes.byDuration.verySlow, traceID)

	s.indexes.byCost.cheap = s.removeString(s.indexes.byCost.cheap, traceID)
	s.indexes.byCost.moderate = s.removeString(s.indexes.byCost.moderate, traceID)
	s.indexes.byCost.expensive = s.removeString(s.indexes.byCost.expensive, traceID)
}

// Helper functions

func (s *MemoryStore) containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func (s *MemoryStore) removeString(slice []string, str string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}

func (s *MemoryStore) deduplicate(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
