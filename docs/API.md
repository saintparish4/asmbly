# TraceFlow API Documentation

**Version**: 1.0.0 (Layer 1 Complete)  
**Base URL**: `http://localhost:9090`

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Error Handling](#error-handling)
4. [Endpoints](#endpoints)
   - [Health & Metrics](#health--metrics)
   - [Span Ingestion](#span-ingestion)
   - [Trace Querying](#trace-querying)
5. [Data Models](#data-models)
6. [Examples](#examples)

---

## Overview

TraceFlow provides a RESTful HTTP API for distributed tracing. The API supports:
- **Span ingestion**: Submit single or batch spans
- **Trace retrieval**: Get complete traces by ID
- **Trace search**: Query traces with filters and pagination
- **Service discovery**: List all traced services
- **Monitoring**: Health checks and Prometheus metrics

**Performance**:
- Ingestion: 5,000+ requests/sec
- Storage: 65,000+ spans/sec
- Query latency: <20ms average

---

## Authentication

**Current**: No authentication required (demo system)

**Production**: Would implement API keys or JWT tokens:
```
Authorization: Bearer <token>
```

---

## Error Handling

### HTTP Status Codes

| Code | Meaning | When Used |
|------|---------|-----------|
| 200 | OK | Successful GET request |
| 202 | Accepted | Span(s) queued for processing |
| 400 | Bad Request | Invalid JSON or missing required fields |
| 404 | Not Found | Trace ID doesn't exist |
| 405 | Method Not Allowed | Wrong HTTP method |
| 500 | Internal Server Error | Storage or processing error |
| 503 | Service Unavailable | Collector queue full (backpressure) |

### Error Response Format

```json
{
  "error": "description of error"
}
```

**Example**:
```bash
curl -X POST http://localhost:9090/api/v1/spans \
  -d 'invalid json'

# Response: 400 Bad Request
{
  "error": "invalid JSON"
}
```

---

## Endpoints

### Health & Metrics

#### GET /health

Health check endpoint with current statistics.

**Request**:
```bash
curl http://localhost:9090/health
```

**Response**: 200 OK
```json
{
  "status": "healthy",
  "spans_received": 12345,
  "spans_stored": 12340,
  "span_errors": 5
}
```

**Fields**:
- `status`: Always "healthy" if responding
- `spans_received`: Total spans received via API
- `spans_stored`: Total spans successfully stored
- `span_errors`: Total span processing errors

---

#### GET /metrics

Prometheus-compatible metrics endpoint.

**Request**:
```bash
curl http://localhost:9090/metrics
```

**Response**: 200 OK (text/plain)
```
# HELP traceflow_spans_received_total Total number of spans received
# TYPE traceflow_spans_received_total counter
traceflow_spans_received_total 12345

# HELP traceflow_spans_stored_total Total number of spans stored
# TYPE traceflow_spans_stored_total counter
traceflow_spans_stored_total 12340

# HELP traceflow_span_errors_total Total number of span errors
# TYPE traceflow_span_errors_total counter
traceflow_span_errors_total 5
```

---

### Span Ingestion

#### POST /api/v1/spans

Submit a single span for processing.

**Request**:
```bash
curl -X POST http://localhost:9090/api/v1/spans \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "a1b2c3d4e5f6789012345678901234ab",
    "span_id": "1234567890abcdef",
    "service_name": "api-server",
    "operation_name": "GET /users",
    "start_time": "2024-01-15T10:30:00Z",
    "duration": 50000000,
    "status": "ok",
    "span_kind": "server",
    "tags": {
      "http.method": "GET",
      "http.url": "/users",
      "http.status_code": "200"
    }
  }'
```

**Response**: 202 Accepted
```json
{
  "status": "accepted"
}
```

**Required Fields**:
- `trace_id`: 32-character hex string (128-bit)
- `span_id`: 16-character hex string (64-bit)
- `service_name`: String, non-empty
- `operation_name`: String, non-empty
- `start_time`: ISO 8601 timestamp
- `status`: "ok" or "error"

**Optional Fields**:
- `parent_span_id`: 16-character hex string
- `span_kind`: "client" | "server" | "internal" | "producer" | "consumer"
- `status_message`: Error details (if status="error")
- `tags`: Key-value pairs
- `deployment_id`: Deployment version identifier
- `git_sha`: Git commit hash
- `environment`: "prod" | "staging" | etc.
- `duration`: Nanoseconds (int64)
- `cost`: Float64 (Week 3 feature)
- `has_profile`: Boolean (Week 3 feature)
- `profile_id`: String (Week 3 feature)

**Error Responses**:
- 400 Bad Request: Invalid JSON or validation failure
- 503 Service Unavailable: Queue full (retry with backoff)

---

#### POST /api/v1/spans/batch

Submit multiple spans in a single request.

**Request**:
```bash
curl -X POST http://localhost:9090/api/v1/spans/batch \
  -H "Content-Type: application/json" \
  -d '[
    {
      "trace_id": "a1b2c3d4e5f6789012345678901234ab",
      "span_id": "1111111111111111",
      "service_name": "frontend",
      "operation_name": "page-load",
      "start_time": "2024-01-15T10:30:00Z",
      "duration": 100000000,
      "status": "ok"
    },
    {
      "trace_id": "a1b2c3d4e5f6789012345678901234ab",
      "span_id": "2222222222222222",
      "service_name": "api",
      "operation_name": "GET /users",
      "start_time": "2024-01-15T10:30:00.01Z",
      "duration": 50000000,
      "status": "ok"
    }
  ]'
```

**Response**: 202 Accepted
```json
{
  "accepted": 2,
  "failed": 0,
  "total": 2
}
```

**Response**: 206 Partial Content (if some spans failed)
```json
{
  "accepted": 1,
  "failed": 1,
  "total": 2
}
```

---

### Trace Querying

#### GET /api/v1/traces/:id

Retrieve a complete trace by ID.

**Request**:
```bash
curl http://localhost:9090/api/v1/traces/a1b2c3d4e5f6789012345678901234ab
```

**Response**: 200 OK
```json
{
  "trace_id": "a1b2c3d4e5f6789012345678901234ab",
  "spans": [
    {
      "trace_id": "a1b2c3d4e5f6789012345678901234ab",
      "span_id": "1111111111111111",
      "service_name": "frontend",
      "operation_name": "page-load",
      "start_time": "2024-01-15T10:30:00Z",
      "duration": 100000000,
      "status": "ok",
      "tags": {}
    },
    {
      "trace_id": "a1b2c3d4e5f6789012345678901234ab",
      "span_id": "2222222222222222",
      "parent_span_id": "1111111111111111",
      "service_name": "api",
      "operation_name": "GET /users",
      "start_time": "2024-01-15T10:30:00.01Z",
      "duration": 50000000,
      "status": "ok",
      "tags": {}
    }
  ],
  "start_time": "2024-01-15T10:30:00Z",
  "duration": 100000000,
  "services": ["api", "frontend"],
  "deployments": {
    "frontend": "v1.2.0-abc123",
    "api": "v2.3.1-def456"
  },
  "total_cost": 0.00015,
  "cost_breakdown": {
    "frontend": 0.0001,
    "api": 0.00005
  }
}
```

**Response**: 404 Not Found
```
Trace not found
```

---

#### GET /api/v1/traces

Search traces with filters and pagination.

**Query Parameters**:

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `service` | string | Filter by service name | `api` |
| `min_duration` | duration | Minimum duration | `100ms`, `1s` |
| `max_duration` | duration | Maximum duration | `500ms`, `2s` |
| `min_cost` | float | Minimum cost | `0.001` |
| `max_cost` | float | Maximum cost | `0.01` |
| `start_time` | RFC3339 | Start of time range | `2024-01-15T10:00:00Z` |
| `end_time` | RFC3339 | End of time range | `2024-01-15T11:00:00Z` |
| `limit` | int | Max results (default 100) | `20` |
| `offset` | int | Skip N results | `40` |

**Duration Format**: Number + unit (ns, us, ms, s, m, h)
- Examples: `50ms`, `1.5s`, `100us`, `2m`

**Request**:
```bash
# All traces for "api" service
curl "http://localhost:9090/api/v1/traces?service=api"

# Slow traces (>100ms)
curl "http://localhost:9090/api/v1/traces?min_duration=100ms"

# Traces from last hour
curl "http://localhost:9090/api/v1/traces?start_time=2024-01-15T10:00:00Z&end_time=2024-01-15T11:00:00Z"

# Paginated results
curl "http://localhost:9090/api/v1/traces?limit=20&offset=0"

# Complex query
curl "http://localhost:9090/api/v1/traces?service=api&min_duration=50ms&max_duration=500ms&limit=10"
```

**Response**: 200 OK
```json
{
  "traces": [
    {
      "trace_id": "...",
      "spans": [...],
      "start_time": "2024-01-15T10:30:00Z",
      "duration": 150000000,
      "services": ["api", "database"],
      "total_cost": 0.00025
    }
  ],
  "total": 1,
  "query": {
    "service": "api",
    "min_duration": 50000000,
    "max_duration": 500000000,
    "limit": 10,
    "offset": 0
  }
}
```

**Pagination Example**:
```bash
# Page 1 (results 0-19)
curl "http://localhost:9090/api/v1/traces?limit=20&offset=0"

# Page 2 (results 20-39)
curl "http://localhost:9090/api/v1/traces?limit=20&offset=20"

# Page 3 (results 40-59)
curl "http://localhost:9090/api/v1/traces?limit=20&offset=40"
```

---

#### GET /api/v1/services

List all unique service names.

**Request**:
```bash
curl http://localhost:9090/api/v1/services
```

**Response**: 200 OK
```json
{
  "services": [
    "api",
    "database",
    "frontend"
  ],
  "total": 3
}
```

**Note**: Services are sorted alphabetically.

---

## Data Models

### Span

```json
{
  "trace_id": "string (32 hex chars)",
  "span_id": "string (16 hex chars)",
  "parent_span_id": "string (16 hex chars, optional)",
  "service_name": "string",
  "operation_name": "string",
  "start_time": "ISO 8601 timestamp",
  "duration": "int64 (nanoseconds)",
  "span_kind": "client|server|internal|producer|consumer",
  "status": "ok|error",
  "status_message": "string (optional)",
  "tags": {
    "key": "value"
  },
  "deployment_id": "string (optional)",
  "git_sha": "string (optional)",
  "environment": "string (optional)",
  "cost": "float64 (optional)",
  "has_profile": "boolean (optional)",
  "profile_id": "string (optional)"
}
```

### Trace

```json
{
  "trace_id": "string",
  "spans": [Span],
  "start_time": "ISO 8601 timestamp",
  "duration": "int64 (nanoseconds)",
  "services": ["string"],
  "deployments": {
    "service_name": "deployment_id"
  },
  "total_cost": "float64",
  "cost_breakdown": {
    "service_name": "float64"
  }
}
```

---

## Examples

### Complete End-to-End Example

```bash
# 1. Start collector
go run cmd/collector/main.go

# 2. Submit a trace with 3 spans
TRACE_ID="a1b2c3d4e5f6789012345678901234ab"

# Frontend span
curl -X POST http://localhost:9090/api/v1/spans \
  -H "Content-Type: application/json" \
  -d "{
    \"trace_id\": \"$TRACE_ID\",
    \"span_id\": \"1111111111111111\",
    \"service_name\": \"frontend\",
    \"operation_name\": \"page-load\",
    \"start_time\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
    \"duration\": 150000000,
    \"status\": \"ok\",
    \"span_kind\": \"client\"
  }"

# API span
curl -X POST http://localhost:9090/api/v1/spans \
  -H "Content-Type: application/json" \
  -d "{
    \"trace_id\": \"$TRACE_ID\",
    \"span_id\": \"2222222222222222\",
    \"parent_span_id\": \"1111111111111111\",
    \"service_name\": \"api\",
    \"operation_name\": \"GET /users\",
    \"start_time\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
    \"duration\": 100000000,
    \"status\": \"ok\",
    \"span_kind\": \"server\"
  }"

# Database span
curl -X POST http://localhost:9090/api/v1/spans \
  -H "Content-Type: application/json" \
  -d "{
    \"trace_id\": \"$TRACE_ID\",
    \"span_id\": \"3333333333333333\",
    \"parent_span_id\": \"2222222222222222\",
    \"service_name\": \"database\",
    \"operation_name\": \"SELECT users\",
    \"start_time\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
    \"duration\": 50000000,
    \"status\": \"ok\",
    \"span_kind\": \"client\",
    \"tags\": {
      \"db.system\": \"postgresql\",
      \"db.statement\": \"SELECT * FROM users\"
    }
  }"

# 3. Wait for processing
sleep 1

# 4. Retrieve the trace
curl http://localhost:9090/api/v1/traces/$TRACE_ID | jq '.'

# 5. Query by service
curl "http://localhost:9090/api/v1/traces?service=frontend" | jq '.'

# 6. Get all services
curl http://localhost:9090/api/v1/services | jq '.'

# 7. Check health
curl http://localhost:9090/health | jq '.'
```

### Batch Submission Example

```bash
curl -X POST http://localhost:9090/api/v1/spans/batch \
  -H "Content-Type: application/json" \
  -d '[
    {
      "trace_id": "trace1111111111111111111111111111",
      "span_id": "span1111111111",
      "service_name": "service-a",
      "operation_name": "op-a",
      "start_time": "2024-01-15T10:00:00Z",
      "duration": 10000000,
      "status": "ok"
    },
    {
      "trace_id": "trace2222222222222222222222222222",
      "span_id": "span2222222222",
      "service_name": "service-b",
      "operation_name": "op-b",
      "start_time": "2024-01-15T10:00:01Z",
      "duration": 20000000,
      "status": "ok"
    }
  ]'
```

### Error Handling Example

```bash
# Invalid span (missing required field)
curl -X POST http://localhost:9090/api/v1/spans \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "abc123",
    "span_id": "def456"
  }'

# Response: 400 Bad Request
# (Missing service_name, operation_name, etc.)
```

---

## Rate Limits

**Current**: No rate limiting (demo system)

**Production**: Would implement:
- Per-IP: 1000 requests/minute
- Per-API-key: 10,000 requests/minute

**Response** when rate limited:
```
HTTP/1.1 429 Too Many Requests
Retry-After: 60

{
  "error": "rate limit exceeded"
}
```

---

## CORS

All endpoints support CORS with:
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type
```

**Preflight Request**:
```bash
curl -X OPTIONS http://localhost:9090/api/v1/spans \
  -H "Origin: http://example.com"

# Response: 200 OK with CORS headers
```

---

## Client Libraries

**Current**: None (use HTTP directly)

**Future**: Official SDKs for:
- Go
- Python
- Node.js
- Java
- Ruby

---

## Changelog

### Version 1.0.0 (Layer 1 Complete)
- ✅ Span ingestion (single & batch)
- ✅ Trace retrieval by ID
- ✅ Trace search with filters
- ✅ Service discovery
- ✅ Health checks & metrics
- ✅ CORS support
- ✅ Graceful shutdown

### Upcoming (Week 2+)
- Instrumentation SDK
- Demo microservices
- Web UI with timeline visualization
- Cost attribution
- Trace-to-code profiling
- Architectural drift detection

---

## Support

**Issues**: https://github.com/saint/traceflow/issues  
**Documentation**: https://github.com/saint/traceflow/docs  
**Demo**: See `scripts/test-collector.sh`

---

**API Version**: 1.0.0  
**Last Updated**: 2024-01-15