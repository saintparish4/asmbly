package instrumentation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// W3C Trace Context constants
const (
	// TraceParentHeader is the W3C traceparent header name
	TraceParentHeader = "traceparent"

	// TraceStateHeader is the W3C tracestate header name
	TraceStateHeader = "tracestate"
)

// TraceContext represents parsed W3C Trace Context.
type TraceContext struct {
	Version string
	TraceID string
	SpanID  string
	Flags   string
}

// W3C Trace Context format: version-trace-id-parent-id-trace-flags
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
var traceParentRegex = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)

// EncodeTraceParent creates a W3C traceparent header value.
// Format: version-trace-id-parent-id-trace-flags
func EncodeTraceParent(traceID, spanID, flags string) string {
	// Default version is "00"
	version := "00"

	// Default flags is "01" (sampled)
	if flags == "" {
		flags = "01"
	}

	return fmt.Sprintf("%s-%s-%s-%s", version, traceID, spanID, flags)
}

// DecodeTraceParent parses a W3C traceparent header.
func DecodeTraceParent(header string) (*TraceContext, error) {
	if header == "" {
		return nil, fmt.Errorf("traceparent header is empty")
	}

	// Match against W3C format
	matches := traceParentRegex.FindStringSubmatch(header)
	if matches == nil {
		return nil, fmt.Errorf("invalid traceparent format: %s", header)
	}

	return &TraceContext{
		Version: matches[1],
		TraceID: matches[2],
		SpanID:  matches[3],
		Flags:   matches[4],
	}, nil
}

// IsValidTraceParent checks if a header value is valid W3C format.
func IsValidTraceParent(header string) bool {
	return traceParentRegex.MatchString(header)
}

// Context helpers

type contextKey int

const (
	spanContextKey contextKey = iota
	traceContextContextKey
)

// SpanFromContext extracts the span from the context.
func SpanFromContext(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}

	span, ok := ctx.Value(spanContextKey).(*Span)
	if !ok {
		return nil
	}

	return span
}

// ContextWithSpan adds a span to the context.
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, spanContextKey, span)
}

// traceContextFromContext extracts trace context from the context.
func traceContextFromContext(ctx context.Context) *TraceContext {
	if ctx == nil {
		return nil
	}

	tc, ok := ctx.Value(traceContextContextKey).(*TraceContext)
	if !ok {
		return nil
	}

	return tc
}

// contextWithTraceContext adds trace context to the context.
func contextWithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceContextContextKey, tc)
}

// InjectTraceContext injects trace context into HTTP headers.
func InjectTraceContext(span *Span, header func(key, value string)) {
	if span == nil || span.span == nil {
		return
	}

	// Create traceparent header
	traceparent := EncodeTraceParent(span.span.TraceID, span.span.SpanID, "01")
	header(TraceParentHeader, traceparent)
}

// ExtractTraceContext extracts trace context from HTTP headers.
func ExtractTraceContext(getHeader func(key string) string) (*TraceContext, error) {
	// Get traceparent header
	traceparent := getHeader(TraceParentHeader)
	if traceparent == "" {
		// Try lowercase (some frameworks lowercase headers)
		traceparent = getHeader(strings.ToLower(TraceParentHeader))
	}

	if traceparent == "" {
		return nil, nil // No trace context
	}

	// Parse header
	return DecodeTraceParent(traceparent)
}
