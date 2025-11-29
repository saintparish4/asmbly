package instrumentation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/saintparish4/asmbly/internal/models"
)

// Tracer is the main entry point for instrumentation
// It provides methods to create and manage spans
type Tracer struct {
	serviceName  string
	collectorUrl string
	client       *http.Client
	sampler      Sampler
	logger       *slog.Logger
}

// Sampler determines whether a span should be sampled
type Sampler interface {
	ShouldSample(operationName string) bool
}

// AlwaysSampler samples every span
type AlwaysSampler struct{}

func (s *AlwaysSampler) ShouldSample(operationName string) bool {
	return true
}

// Span represents an active span in the SDK
// It wraps the underlying models.Span and provides methods to manage it
type Span struct {
	tracer    *Tracer
	span      *models.Span
	startTime time.Time
}

// Option is a function that configures a span
type Option func(*Span)

// NewTracer creates a new tracer for the given service
func NewTracer(serviceName, collectorUrl string) *Tracer {
	return &Tracer{
		serviceName:  serviceName,
		collectorUrl: collectorUrl,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		sampler: &AlwaysSampler{},
		logger:  slog.Default(),
	}
}

// WithHTTPClient sets a custom http client
func (t *Tracer) WithHTTPClient(client *http.Client) *Tracer {
	t.client = client
	return t
}

// WithSampler sets a custom sampler
func (t *Tracer) WithSampler(sampler Sampler) *Tracer {
	t.sampler = sampler
	return t
}

// WithLogger sets a custom logger
func (t *Tracer) WithLogger(logger *slog.Logger) *Tracer {
	t.logger = logger
	return t
}

// StartSpan creates and starts a new span
func (t *Tracer) StartSpan(ctx context.Context, operationName string, opts ...Option) (*Span, context.Context) {
	// Check sampling
	if !t.sampler.ShouldSample(operationName) {
		// Return a no-op span
		return &Span{tracer: t}, ctx
	}

	// Get or create trace ID
	var traceID string
	var parentSpanID string

	// Try to get parent span from context
	if parent := SpanFromContext(ctx); parent != nil && parent.span != nil {
		traceID = parent.span.TraceID
		parentSpanID = parent.span.SpanID
	} else {
		// Try to extract from W3C Trace Context in context
		if tc := traceContextFromContext(ctx); tc != nil {
			traceID = tc.TraceID
			parentSpanID = tc.SpanID
		} else {
			// CREATE NEW TRACE
			traceID = models.GenerateTraceID()
		}
	}

	// Create span
	span := &Span{
		tracer:    t,
		startTime: time.Now(),
		span: &models.Span{
			TraceID:       traceID,
			SpanID:        models.GenerateSpanID(),
			ParentSpanID:  parentSpanID,
			ServiceName:   t.serviceName,
			OperationName: operationName,
			StartTime:     time.Now(),
			SpanKind:      "internal", // Default
			Status:        "ok",       // Default
			Tags:          make(map[string]string),
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(span)
	}

	// Add span to context
	ctx = ContextWithSpan(ctx, span)

	return span, ctx
}

// Finish completes the span and sends it to the collector asynchronously.
func (s *Span) Finish() {
	if s.span == nil {
		return // No-op span
	}

	// Calculate duration
	s.span.Duration = time.Since(s.startTime)

	// Send span asynchronously (don't block)
	go s.tracer.sendSpan(s.span)
}

// SetTag adds a tag to the span.
func (s *Span) SetTag(key, value string) *Span {
	if s.span != nil {
		s.span.SetTag(key, value)
	}
	return s
}

// SetError marks the span as failed and records the error.
func (s *Span) SetError(err error) *Span {
	if s.span != nil && err != nil {
		s.span.Status = "error"
		s.span.StatusMessage = err.Error()
		s.span.SetTag("error", "true")
		s.span.SetTag("error.message", err.Error())
	}
	return s
}

// SetStatus sets the span status.
func (s *Span) SetStatus(status string) *Span {
	if s.span != nil {
		s.span.Status = status
	}
	return s
}

// SetSpanKind sets the span kind.
func (s *Span) SetSpanKind(kind string) *Span {
	if s.span != nil {
		s.span.SpanKind = kind
	}
	return s
}

// TraceID returns the trace ID of this span.
func (s *Span) TraceID() string {
	if s.span != nil {
		return s.span.TraceID
	}
	return ""
}

// SpanID returns the span ID of this span.
func (s *Span) SpanID() string {
	if s.span != nil {
		return s.span.SpanID
	}
	return ""
}

// sendSpan sends a span to the collector.
// This is called asynchronously and should not block.
func (t *Tracer) sendSpan(span *models.Span) {
	// Marshal span to JSON
	data, err := json.Marshal(span)
	if err != nil {
		t.logger.Error("failed to marshal span", "error", err)
		return
	}

	// Send to collector
	url := fmt.Sprintf("%s/api/v1/spans", t.collectorUrl)
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.logger.Error("failed to send span",
			"trace_id", span.TraceID,
			"span_id", span.SpanID,
			"error", err,
		)
		return
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		t.logger.Warn("collector returned non-2xx status",
			"status", resp.StatusCode,
			"trace_id", span.TraceID,
			"span_id", span.SpanID,
		)
	}
}

// Option functions

// WithParent sets the parent span.
func WithParent(parent *Span) Option {
	return func(s *Span) {
		if parent != nil && parent.span != nil {
			s.span.TraceID = parent.span.TraceID
			s.span.ParentSpanID = parent.span.SpanID
		}
	}
}

// WithTags sets multiple tags on the span.
func WithTags(tags map[string]string) Option {
	return func(s *Span) {
		if s.span != nil {
			for k, v := range tags {
				s.span.SetTag(k, v)
			}
		}
	}
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind string) Option {
	return func(s *Span) {
		if s.span != nil {
			s.span.SpanKind = kind
		}
	}
}

// WithDeployment sets deployment information.
func WithDeployment(deploymentID, gitSHA, environment string) Option {
	return func(s *Span) {
		if s.span != nil {
			s.span.DeploymentID = deploymentID
			s.span.GitSHA = gitSHA
			s.span.Environment = environment
		}
	}
}

// WithProfiling enables profiling for this span (Later).
func WithProfiling() Option {
	return func(s *Span) {
		if s.span != nil {
			s.span.HasProfile = true
			// Profiling will be implemented in Later Weeks
		}
	}
}
