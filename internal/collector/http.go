package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
	"github.com/saintparish4/asmbly/internal/storage"
)

// Collector receives and processes spans using a worker pool pattern
// It provides HTTP endpoints for span ingestion and trace querying
type Collector struct {
	store   storage.Store
	spanCh  chan *models.Span // Buffered channel for async processing
	workers int               // Number of worker goroutines
	wg      sync.WaitGroup    // Wait for workers to finish

	// Metrics
	metrics *Metrics

	// Lifecycle
	stopCh chan struct{}
	logger *slog.Logger
}

// Metrics tracks collector statistics
type Metrics struct {
	SpansReceived int64
	SpansStored   int64
	SpanErrors    int64
	mu            sync.Mutex
}

// Config holds collector configuration.
type Config struct {
	Workers       int
	ChannelBuffer int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Workers:       10,
		ChannelBuffer: 1000,
	}
}

// NewCollector creates a new span collector with the given configuration.
func NewCollector(store storage.Store, config *Config, logger *slog.Logger) *Collector {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Collector{
		store:   store,
		spanCh:  make(chan *models.Span, config.ChannelBuffer),
		workers: config.Workers,
		metrics: &Metrics{},
		stopCh:  make(chan struct{}),
		logger:  logger,
	}
}

// Start begins processing spans with worker goroutines.
// This must be called before the collector can accept spans.
func (c *Collector) Start(ctx context.Context) {
	c.logger.Info("starting collector workers", "workers", c.workers)

	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.spanWorker(ctx, i)
	}
}

// Stop gracefully shuts down the collector, waiting for in-flight spans to complete.
func (c *Collector) Stop(ctx context.Context) error {
	c.logger.Info("stopping collector")

	// Signal workers to stop
	close(c.stopCh)

	// Close span channel (no more incoming spans)
	close(c.spanCh)

	// Wait for workers to finish processing remaining spans
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("all workers stopped gracefully")
	case <-ctx.Done():
		c.logger.Warn("shutdown timeout, some spans may be lost")
		return ctx.Err()
	}

	return nil
}

// spanWorker processes spans from the channel.
func (c *Collector) spanWorker(ctx context.Context, id int) {
	defer c.wg.Done()

	c.logger.Debug("worker started", "worker_id", id)

	for {
		select {
		case <-c.stopCh:
			// Shutdown requested - drain remaining spans from channel
			c.logger.Debug("worker draining remaining spans", "worker_id", id)
			for span := range c.spanCh {
				if err := c.processSpan(ctx, span); err != nil {
					c.logger.Error("failed to process span",
						"worker_id", id,
						"trace_id", span.TraceID,
						"span_id", span.SpanID,
						"error", err,
					)
					c.metrics.mu.Lock()
					c.metrics.SpanErrors++
					c.metrics.mu.Unlock()
				} else {
					c.metrics.mu.Lock()
					c.metrics.SpansStored++
					c.metrics.mu.Unlock()
				}
			}
			c.logger.Debug("worker stopped", "worker_id", id)
			return
		case span, ok := <-c.spanCh:
			if !ok {
				// Channel closed
				c.logger.Debug("worker exiting (channel closed)", "worker_id", id)
				return
			}

			// Process span
			if err := c.processSpan(ctx, span); err != nil {
				c.logger.Error("failed to process span",
					"worker_id", id,
					"trace_id", span.TraceID,
					"span_id", span.SpanID,
					"error", err,
				)
				c.metrics.mu.Lock()
				c.metrics.SpanErrors++
				c.metrics.mu.Unlock()
			} else {
				c.metrics.mu.Lock()
				c.metrics.SpansStored++
				c.metrics.mu.Unlock()
			}
		}
	}
}

// processSpan validates and stores a single span.
func (c *Collector) processSpan(ctx context.Context, span *models.Span) error {
	// Validate span (storage will also validate, but fail fast here)
	if err := span.Validate(); err != nil {
		return fmt.Errorf("invalid span: %w", err)
	}

	// Store span
	if err := c.store.WriteSpan(ctx, span); err != nil {
		return fmt.Errorf("failed to store span: %w", err)
	}

	return nil
}

// SubmitSpan adds a span to the processing queue.
// This is non-blocking - the span is processed asynchronously by workers.
func (c *Collector) SubmitSpan(span *models.Span) error {
	select {
	case c.spanCh <- span:
		c.metrics.mu.Lock()
		c.metrics.SpansReceived++
		c.metrics.mu.Unlock()
		return nil
	case <-c.stopCh:
		return fmt.Errorf("collector is stopping")
	default:
		// Channel full - this is a backpressure signal
		return fmt.Errorf("span queue full, try again later")
	}
}

// GetMetrics returns a snapshot of current metrics.
func (c *Collector) GetMetrics() Metrics {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	return Metrics{
		SpansReceived: c.metrics.SpansReceived,
		SpansStored:   c.metrics.SpansStored,
		SpanErrors:    c.metrics.SpanErrors,
	}
}

// HTTP Handlers

// HandlePostSpan handles POST /api/v1/spans - submit a single span.
func (c *Collector) HandlePostSpan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and parse span
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var span models.Span
	if err := json.Unmarshal(body, &span); err != nil {
		c.logger.Error("failed to parse span JSON", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Submit span
	if err := c.SubmitSpan(&span); err != nil {
		c.logger.Error("failed to submit span", "error", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
	})
}

// HandlePostSpansBatch handles POST /api/v1/spans/batch - submit multiple spans.
func (c *Collector) HandlePostSpansBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and parse spans
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var spans []models.Span
	if err := json.Unmarshal(body, &spans); err != nil {
		c.logger.Error("failed to parse spans JSON", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Submit all spans
	accepted := 0
	failed := 0
	for i := range spans {
		if err := c.SubmitSpan(&spans[i]); err != nil {
			c.logger.Warn("failed to submit span in batch",
				"span_index", i,
				"error", err,
			)
			failed++
		} else {
			accepted++
		}
	}

	// Response
	w.Header().Set("Content-Type", "application/json")
	if failed > 0 {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accepted": accepted,
		"failed":   failed,
		"total":    len(spans),
	})
}

// HandleGetTrace handles GET /api/v1/traces/:id - retrieve a trace by ID.
func (c *Collector) HandleGetTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract trace ID from path (simple parsing - no router needed)
	traceID := r.URL.Path[len("/api/v1/traces/"):]
	if traceID == "" {
		http.Error(w, "trace ID required", http.StatusBadRequest)
		return
	}

	// Get trace
	trace, err := c.store.GetTrace(r.Context(), traceID)
	if err != nil {
		c.logger.Error("failed to get trace", "trace_id", traceID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if trace == nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trace)
}

// HandleFindTraces handles GET /api/v1/traces - search traces with filters.
func (c *Collector) HandleFindTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := c.parseQuery(r)

	// Execute query
	start := time.Now()
	traces, err := c.store.FindTraces(r.Context(), query)
	if err != nil {
		c.logger.Error("failed to find traces", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	duration := time.Since(start)

	c.logger.Debug("query executed",
		"duration_ms", duration.Milliseconds(),
		"results", len(traces),
	)

	// Success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"traces": traces,
		"total":  len(traces),
		"query":  query,
	})
}

// HandleGetServices handles GET /api/v1/services - list all services.
func (c *Collector) HandleGetServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get services
	services, err := c.store.GetServices(r.Context())
	if err != nil {
		c.logger.Error("failed to get services", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
		"total":    len(services),
	})
}

// parseQuery parses URL query parameters into a storage.Query.
func (c *Collector) parseQuery(r *http.Request) *storage.Query {
	query := storage.NewQuery()

	// Service filter
	if service := r.URL.Query().Get("service"); service != "" {
		query.Service = service
	}

	// Duration filters
	if minDur := r.URL.Query().Get("min_duration"); minDur != "" {
		if d, err := time.ParseDuration(minDur); err == nil {
			query.MinDuration = d
		}
	}
	if maxDur := r.URL.Query().Get("max_duration"); maxDur != "" {
		if d, err := time.ParseDuration(maxDur); err == nil {
			query.MaxDuration = d
		}
	}

	// Cost filters (Week 3)
	if minCost := r.URL.Query().Get("min_cost"); minCost != "" {
		if f, err := strconv.ParseFloat(minCost, 64); err == nil {
			query.MinCost = f
		}
	}
	if maxCost := r.URL.Query().Get("max_cost"); maxCost != "" {
		if f, err := strconv.ParseFloat(maxCost, 64); err == nil {
			query.MaxCost = f
		}
	}

	// Time range filters
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			query.StartTime = t
		}
	}
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			query.EndTime = t
		}
	}

	// Pagination
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			query.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			query.Offset = o
		}
	}

	return query
}

// Middleware

// CORSMiddleware adds CORS headers for cross-origin requests.
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(logger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call next handler
		next(w, r)

		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
