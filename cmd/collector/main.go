package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Enable pprof endpoints
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saintparish4/asmbly/internal/collector"
	"github.com/saintparish4/asmbly/internal/storage"
)

// Config holds application configuration.
type Config struct {
	Port       int
	Workers    int
	LogLevel   string
	MaxTraces  int
	BufferSize int
}

func main() {
	// Parse configuration
	config := parseConfig()

	// Setup logger
	logger := setupLogger(config.LogLevel)
	logger.Info("starting traceflow collector",
		"port", config.Port,
		"workers", config.Workers,
		"max_traces", config.MaxTraces,
	)

	// Initialize storage
	store := storage.NewMemoryStore(config.MaxTraces)
	logger.Info("storage initialized", "type", "in-memory", "max_traces", config.MaxTraces)

	// Initialize collector
	collectorConfig := &collector.Config{
		Workers:       config.Workers,
		ChannelBuffer: config.BufferSize,
	}
	col := collector.NewCollector(store, collectorConfig, logger)

	// Start collector workers
	ctx := context.Background()
	col.Start(ctx)
	logger.Info("collector workers started", "count", config.Workers)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Span ingestion endpoints
	mux.HandleFunc("/api/v1/spans",
		collector.CORSMiddleware(
			collector.LoggingMiddleware(logger, col.HandlePostSpan),
		),
	)
	mux.HandleFunc("/api/v1/spans/batch",
		collector.CORSMiddleware(
			collector.LoggingMiddleware(logger, col.HandlePostSpansBatch),
		),
	)

	// Trace query endpoints
	mux.HandleFunc("/api/v1/traces/",
		collector.CORSMiddleware(
			collector.LoggingMiddleware(logger, col.HandleGetTrace),
		),
	)
	mux.HandleFunc("/api/v1/traces",
		collector.CORSMiddleware(
			collector.LoggingMiddleware(logger, col.HandleFindTraces),
		),
	)

	// Services endpoint
	mux.HandleFunc("/api/v1/services",
		collector.CORSMiddleware(
			collector.LoggingMiddleware(logger, col.HandleGetServices),
		),
	)

	// Health check endpoint
	mux.HandleFunc("/health", handleHealth(col))

	// Metrics endpoint (Prometheus-compatible)
	mux.HandleFunc("/metrics", handleMetrics(col))

	// Create HTTP server
	addr := fmt.Sprintf(":%d", config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start pprof server on port 6060 (for profiling)
	pprofServer := &http.Server{
		Addr:    ":6060",
		Handler: http.DefaultServeMux, // pprof registers with DefaultServeMux
	}
	pprofErrors := make(chan error, 1)
	go func() {
		logger.Info("pprof server listening", "addr", ":6060")
		pprofErrors <- pprofServer.ListenAndServe()
	}()

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error", "error", err)
		os.Exit(1)
	case err := <-pprofErrors:
		logger.Error("pprof server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig)

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop pprof server
		if err := pprofServer.Shutdown(ctx); err != nil {
			logger.Error("pprof server shutdown error", "error", err)
			pprofServer.Close()
		}

		// Stop accepting new requests
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("http server shutdown error", "error", err)
			server.Close()
		}

		// Stop collector workers (drain in-flight spans)
		if err := col.Stop(ctx); err != nil {
			logger.Error("collector shutdown error", "error", err)
		}

		// Close storage
		if err := store.Close(); err != nil {
			logger.Error("storage close error", "error", err)
		}

		logger.Info("shutdown complete")
	}
}

// parseConfig parses configuration from command-line flags and environment variables.
func parseConfig() *Config {
	config := &Config{}

	// Define flags
	flag.IntVar(&config.Port, "port", getEnvInt("PORT", 9090), "HTTP server port")
	flag.IntVar(&config.Workers, "workers", getEnvInt("WORKERS", 10), "Number of worker goroutines")
	flag.StringVar(&config.LogLevel, "log-level", getEnvString("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	flag.IntVar(&config.MaxTraces, "max-traces", getEnvInt("MAX_TRACES", 10000), "Maximum traces to keep in memory")
	flag.IntVar(&config.BufferSize, "buffer-size", getEnvInt("BUFFER_SIZE", 1000), "Span channel buffer size")

	flag.Parse()

	return config
}

// setupLogger creates a structured logger with the specified level.
func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level

	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(handler)
}

// handleHealth returns a health check handler.
func handleHealth(col *collector.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := col.GetMetrics()

		status := map[string]interface{}{
			"status":         "healthy",
			"spans_received": metrics.SpansReceived,
			"spans_stored":   metrics.SpansStored,
			"span_errors":    metrics.SpanErrors,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}
}

// handleMetrics returns a Prometheus-compatible metrics handler.
func handleMetrics(col *collector.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := col.GetMetrics()

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Prometheus format
		fmt.Fprintf(w, "# HELP traceflow_spans_received_total Total number of spans received\n")
		fmt.Fprintf(w, "# TYPE traceflow_spans_received_total counter\n")
		fmt.Fprintf(w, "traceflow_spans_received_total %d\n", metrics.SpansReceived)

		fmt.Fprintf(w, "# HELP traceflow_spans_stored_total Total number of spans stored\n")
		fmt.Fprintf(w, "# TYPE traceflow_spans_stored_total counter\n")
		fmt.Fprintf(w, "traceflow_spans_stored_total %d\n", metrics.SpansStored)

		fmt.Fprintf(w, "# HELP traceflow_span_errors_total Total number of span errors\n")
		fmt.Fprintf(w, "# TYPE traceflow_span_errors_total counter\n")
		fmt.Fprintf(w, "traceflow_span_errors_total %d\n", metrics.SpanErrors)
	}
}

// Helper functions for environment variables

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
