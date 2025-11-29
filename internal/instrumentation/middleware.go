package instrumentation

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Middleware creates an HTTP middleware that automatically traces requests.
func Middleware(tracer *Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			tc, _ := ExtractTraceContext(func(key string) string {
				return r.Header.Get(key)
			})

			// Add trace context to request context
			ctx := r.Context()
			if tc != nil {
				ctx = contextWithTraceContext(ctx, tc)
			}

			// Start span for this request
			span, ctx := tracer.StartSpan(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				WithSpanKind("server"),
			)
			defer span.Finish()

			// Set HTTP tags
			span.SetTag("http.method", r.Method)
			span.SetTag("http.url", r.URL.Path)
			span.SetTag("http.host", r.Host)
			span.SetTag("http.scheme", r.URL.Scheme)
			if r.URL.Scheme == "" {
				if r.TLS != nil {
					span.SetTag("http.scheme", "https")
				} else {
					span.SetTag("http.scheme", "http")
				}
			}

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default
				span:           span,
			}

			// Handle panics
			defer func() {
				if err := recover(); err != nil {
					span.SetTag("error", "true")
					span.SetTag("error.message", fmt.Sprintf("panic: %v", err))
					span.SetStatus("error")
					panic(err) // Re-throw panic
				}
			}()

			// Update request with new context
			r = r.WithContext(ctx)

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Set final status code
			span.SetTag("http.status_code", fmt.Sprintf("%d", wrapped.statusCode))

			// Mark as error if status >= 500
			if wrapped.statusCode >= 500 {
				span.SetStatus("error")
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	span       *Span
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write ensures status code is captured even if WriteHeader wasn't called.
func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// RoundTripper wraps http.RoundTripper to inject trace context into outgoing requests.
type RoundTripper struct {
	base http.RoundTripper
}

// RoundTrip injects trace context into the outgoing request.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Get span from context
	span := SpanFromContext(req.Context())
	if span != nil && span.span != nil {
		// Inject trace context into headers
		InjectTraceContext(span, func(key, value string) {
			req.Header.Set(key, value)
		})
	}

	// Execute request
	return rt.base.RoundTrip(req)
}

// WrapHTTPClient wraps an HTTP client to automatically propagate trace context.
func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	// Get base transport
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	// Create wrapped client
	wrapped := *client
	wrapped.Transport = &RoundTripper{base: base}

	return &wrapped
}

// DoRequest is a helper that makes an HTTP request with trace context.
func DoRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	// Add context to request
	req = req.WithContext(ctx)

	// Wrap client if not already wrapped
	wrappedClient := WrapHTTPClient(client)

	// Execute request
	return wrappedClient.Do(req)
}

// ClientMiddleware creates a middleware for HTTP clients that creates a span for each request.
func ClientMiddleware(tracer *Tracer) func(*http.Client) *http.Client {
	return func(client *http.Client) *http.Client {
		if client == nil {
			client = http.DefaultClient
		}

		// Get base transport
		base := client.Transport
		if base == nil {
			base = http.DefaultTransport
		}

		// Create wrapped client
		wrapped := *client
		wrapped.Transport = &tracingRoundTripper{
			base:   base,
			tracer: tracer,
		}

		return &wrapped
	}
}

// tracingRoundTripper creates a span for each HTTP request.
type tracingRoundTripper struct {
	base   http.RoundTripper
	tracer *Tracer
}

// RoundTrip creates a span and injects trace context.
func (rt *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Start span for outgoing request
	span, ctx := rt.tracer.StartSpan(req.Context(),
		fmt.Sprintf("%s %s", req.Method, req.URL.Path),
		WithSpanKind("client"),
	)
	defer span.Finish()

	// Set HTTP tags
	span.SetTag("http.method", req.Method)
	span.SetTag("http.url", req.URL.String())
	span.SetTag("http.host", req.URL.Host)

	// Inject trace context
	InjectTraceContext(span, func(key, value string) {
		req.Header.Set(key, value)
	})

	// Update request context
	req = req.WithContext(ctx)

	// Execute request
	start := time.Now()
	resp, err := rt.base.RoundTrip(req)
	duration := time.Since(start)

	// Record response
	if err != nil {
		span.SetError(err)
	} else {
		span.SetTag("http.status_code", fmt.Sprintf("%d", resp.StatusCode))
		if resp.StatusCode >= 500 {
			span.SetStatus("error")
		}
	}

	span.SetTag("http.duration_ms", fmt.Sprintf("%d", duration.Milliseconds()))

	return resp, err
}
