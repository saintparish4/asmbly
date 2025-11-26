package models

import (
	"errors"
	"time"
)

// Span represents a single unit of work in a distributed trace.
// It includes deployment metadata for per-version performance tracking.
type Span struct {
	// Core tracing identifiers
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`

	// Service context
	ServiceName   string `json:"service_name"`
	OperationName string `json:"operation_name"`

	// Timing
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`

	// W3C Trace Context - describes the role of this span
	// Valid values: "client", "server", "internal", "producer", "consumer"
	SpanKind string `json:"span_kind,omitempty"`

	// Status indicates success or failure
	Status        string `json:"status"` // "ok" or "error"
	StatusMessage string `json:"status_message,omitempty"`

	// Tags are key-value pairs for additional context
	Tags map[string]string `json:"tags,omitempty"`

	// 🚀 Deployment tracking - enables per-version performance analysis
	DeploymentID string `json:"deployment_id,omitempty"` // e.g., "v2.3.1-abc123"
	GitSHA       string `json:"git_sha,omitempty"`       // commit hash
	Environment  string `json:"environment,omitempty"`   // "prod", "staging", etc.

	// Cost attribution (populated in Week 3)
	Cost float64 `json:"cost,omitempty"`

	// Profiling integration (populated in Week 3)
	HasProfile bool   `json:"has_profile,omitempty"`
	ProfileID  string `json:"profile_id,omitempty"`
}

// Trace represents a complete trace containing multiple spans.
type Trace struct {
	TraceID   string        `json:"trace_id"`
	Spans     []Span        `json:"spans"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`

	// Services involved in this trace
	Services []string `json:"services"`

	// Deployment context - maps service name to deployment ID
	Deployments map[string]string `json:"deployments,omitempty"`

	// Cost attribution (populated in Week 3)
	TotalCost     float64            `json:"total_cost,omitempty"`
	CostBreakdown map[string]float64 `json:"cost_breakdown,omitempty"` // service → cost
}

// Common validation errors
var (
	ErrMissingTraceID       = errors.New("trace_id is required")
	ErrMissingSpanID        = errors.New("span_id is required")
	ErrMissingServiceName   = errors.New("service_name is required")
	ErrMissingOperationName = errors.New("operation_name is required")
	ErrInvalidTraceIDFormat = errors.New("trace_id must be 32 hex characters")
	ErrInvalidSpanIDFormat  = errors.New("span_id must be 16 hex characters")
	ErrNegativeDuration     = errors.New("duration cannot be negative")
	ErrMissingStartTime     = errors.New("start_time is required")
	ErrInvalidStatus        = errors.New("status must be 'ok' or 'error'")
	ErrInvalidSpanKind      = errors.New("span_kind must be one of: client, server, internal, producer, consumer")
)

// Validate checks if the span has all required fields and valid values.
// This is called before storing a span to ensure data integrity.
func (s *Span) Validate() error {
	// Required fields
	if s.TraceID == "" {
		return ErrMissingTraceID
	}
	if s.SpanID == "" {
		return ErrMissingSpanID
	}
	if s.ServiceName == "" {
		return ErrMissingServiceName
	}
	if s.OperationName == "" {
		return ErrMissingOperationName
	}

	// Format validation - ensure IDs are properly formatted
	if !IsValidTraceID(s.TraceID) {
		return ErrInvalidTraceIDFormat
	}
	if !IsValidSpanID(s.SpanID) {
		return ErrInvalidSpanIDFormat
	}

	// Logic validation
	if s.Duration < 0 {
		return ErrNegativeDuration
	}
	if s.StartTime.IsZero() {
		return ErrMissingStartTime
	}

	// Status validation
	if s.Status != "ok" && s.Status != "error" {
		return ErrInvalidStatus
	}

	// SpanKind validation (optional field)
	if s.SpanKind != "" {
		validKinds := map[string]bool{
			"client":   true,
			"server":   true,
			"internal": true,
			"producer": true,
			"consumer": true,
		}
		if !validKinds[s.SpanKind] {
			return ErrInvalidSpanKind
		}
	}

	return nil
}

// EndTime calculates when this span ended.
func (s *Span) EndTime() time.Time {
	return s.StartTime.Add(s.Duration)
}

// IsError returns true if this span represents a failed operation.
func (s *Span) IsError() bool {
	return s.Status == "error"
}

// GetTag retrieves a tag value, returning empty string if not found.
func (s *Span) GetTag(key string) string {
	if s.Tags == nil {
		return ""
	}
	return s.Tags[key]
}

// SetTag sets a tag value, initializing the map if necessary.
func (s *Span) SetTag(key, value string) {
	if s.Tags == nil {
		s.Tags = make(map[string]string)
	}
	s.Tags[key] = value
}
