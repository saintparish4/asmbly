package storage

import (
	"context"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
)

// Store defines the interface for trace storage operations
// Implementations must be safe for concurrent use by multiple goroutines
type Store interface {
	// WriteSpan stores a single span and the span will be validated before storage
	// Returns an error if the span is invalid or storage fails
	WriteSpan(ctx context.Context, span *models.Span) error

	// GetTrace retrieves a complete trace by trace ID, assembling all spans
	// Returns nil if the trace is not found
	GetTrace(ctx context.Context, traceID string) (*models.Trace, error)

	// FindTraces searches for traces matching the given query criteria
	// Results are paginated using query.Limit and query.Offset
	FindTraces(ctx context.Context, query *Query) ([]*models.Trace, error)

	// GetService returns a list of all unique service names that have sent spans
	GetServices(ctx context.Context) ([]string, error)

	// Close cleanly shuts down the storage system, flushing any pending writes
	Close() error
}

// Query defines search criteria for finding traces
// All filters are optional - nil/zero values are ignored
type Query struct {
	// Service filters traces that include this service name
	Service string

	// Duration filters
	MinDuration time.Duration // Include traces with duration >= MinDuration
	MaxDuration time.Duration // Include traces with duration <= MaxDuration

	// Close filters
	MinCost float64 // Include traces with cost >= MinCost
	MaxCost float64 // Include traces with cost <= MaxCost

	// Time range filters
	StartTime time.Time // Include traces with start time >= StartTime
	EndTime   time.Time // Include traces with end time <= EndTime

	// Profiling filter
	HasProfile *bool // If set, filter traces by whether they have profiled spans

	// Pagination
	Limit  int // Max number of results to return (0 = no limit)
	Offset int // Number of results to skip (for pagination)

	// Sorting (future feature)
	// SortBy string // "start_time", "duration", "cost"
	// SortOrder string // "asc", "desc"
}

// QueryResult represents a paginated query response.
type QueryResult struct {
	Traces []*models.Trace // Matching traces
	Total  int             // Total matching traces (before pagination)
	Offset int             // Current offset
	Limit  int             // Current limit
}

// NewQuery creates a Query with default pagination settings.
func NewQuery() *Query {
	return &Query{
		Limit: 100, // Default: return up to 100 traces
	}
}

// WithService adds a service name filter.
func (q *Query) WithService(service string) *Query {
	q.Service = service
	return q
}

// WithDurationRange adds duration filters.
func (q *Query) WithDurationRange(min, max time.Duration) *Query {
	q.MinDuration = min
	q.MaxDuration = max
	return q
}

// WithCostRange adds cost filters.
func (q *Query) WithCostRange(min, max float64) *Query {
	q.MinCost = min
	q.MaxCost = max
	return q
}

// WithTimeRange adds time range filters.
func (q *Query) WithTimeRange(start, end time.Time) *Query {
	q.StartTime = start
	q.EndTime = end
	return q
}

// WithPagination sets pagination parameters.
func (q *Query) WithPagination(limit, offset int) *Query {
	q.Limit = limit
	q.Offset = offset
	return q
}
