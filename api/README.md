# CDN Health Metrics API

REST API for querying CDN health check metrics stored in Redis.

## Features

- Latest metrics per target
- Region-wide metrics aggregation
- CORS support for dashboard integration
- Health check endpoint
- Grouped and individual metric queries

## Architecture

```
┌──────────────┐
│   Dashboard  │
│   (React)    │
└──────┬───────┘
       │ HTTP
       ▼
┌──────────────┐     ┌──────────┐
│     API      │────►│  Redis   │
│  (Express)   │     │          │
└──────────────┘     └──────────┘
```

## Setup

### Prerequisites

- Node.js 18+
- Redis running (via docker-compose)

### Installation

```bash
cd api
npm install
```

### Configuration

Environment variables (in root `.env`):
- `API_PORT` - Server port (default: 3000)
- `REDIS_URL` - Redis connection string
- `CORS_ORIGIN` - Allowed origins (default: *)

### Start Server

```bash
npm start
```

For development with auto-reload:
```bash
npm run dev
```

## API Endpoints

Base URL: `http://localhost:3000/api`

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### GET /metrics/target

Get metrics for a specific target URL.

**Query Parameters:**
- `region` (required) - Region identifier (e.g., us-east-1)
- `url` (required) - Target URL
- `limit` (optional) - Number of metrics to return (default: 10)

**Example:**
```bash
curl "http://localhost:3000/api/metrics/target?region=us-east-1&url=https://www.cloudflare.com&limit=5"
```

**Response:**
```json
{
  "region": "us-east-1",
  "target": "https://www.cloudflare.com",
  "count": 5,
  "metrics": [
    {
      "region": "us-east-1",
      "target": "https://www.cloudflare.com",
      "latency_ms": 145,
      "status": 200,
      "timestamp": "2024-01-15T10:30:00Z",
      "error": null
    }
  ]
}
```

### GET /metrics/region

Get all metrics for a region, grouped by target.

**Query Parameters:**
- `region` (required) - Region identifier
- `limit` (optional) - Number of metrics to return (default: 50)

**Example:**
```bash
curl "http://localhost:3000/api/metrics/region?region=us-east-1"
```

**Response:**
```json
{
  "region": "us-east-1",
  "count": 15,
  "targets": 3,
  "metrics": {
    "https://www.cloudflare.com": [
      {
        "region": "us-east-1",
        "target": "https://www.cloudflare.com",
        "latency_ms": 145,
        "status": 200,
        "timestamp": "2024-01-15T10:30:00Z",
        "error": null
      }
    ],
    "https://www.fastly.com": [...]
  }
}
```

### GET /metrics/latest

Get the latest metric for each target in a region.

**Query Parameters:**
- `region` (required) - Region identifier

**Example:**
```bash
curl "http://localhost:3000/api/metrics/latest?region=us-east-1"
```

**Response:**
```json
{
  "region": "us-east-1",
  "count": 3,
  "metrics": [
    {
      "region": "us-east-1",
      "target": "https://www.cloudflare.com",
      "latency_ms": 145,
      "status": 200,
      "timestamp": "2024-01-15T10:30:00Z",
      "error": null
    }
  ]
}
```

## Testing

```bash
npm test
```

Coverage targets: 60-70% of core functionality

## CORS Configuration

By default, CORS is enabled for all origins (`*`). For production, set specific origins:

```env
CORS_ORIGIN=https://dashboard.example.com
```

Multiple origins (comma-separated):
```env
CORS_ORIGIN=https://dashboard.example.com,https://admin.example.com
```

## Module Overview

### `src/redis.js`
- Redis client management
- Metric query functions
- Connection lifecycle

### `src/routes.js`
- API endpoint definitions
- Request validation
- Response formatting

### `src/index.js`
- Express server setup
- Middleware configuration
- Graceful shutdown handling

## Error Handling

### 400 Bad Request
Missing required parameters

### 500 Internal Server Error
Redis connection issues or query failures

## Performance Considerations

- Redis `KEYS` command is used (acceptable for Layer 0)
- In production (Layer 6), use `SCAN` for large datasets
- Consider caching frequently accessed metrics
- Set appropriate `limit` parameters to control response size

## Integration with Dashboard

The API is designed to work with the Layer 4 dashboard:

```javascript
// React example
const fetchLatestMetrics = async (region) => {
  const response = await fetch(
    `http://localhost:3000/api/metrics/latest?region=${region}`
  );
  return response.json();
};
```

## Troubleshooting

### Redis Connection Failed
- Ensure Redis is running: `docker ps`
- Check `REDIS_URL` in `.env`
- Test connection: `redis-cli ping`

### Empty Response
- Verify worker is running and storing metrics
- Check Redis for data: `redis-cli KEYS "metrics:*"`
- Confirm region name matches worker configuration

### CORS Errors
- Check `CORS_ORIGIN` setting
- Verify dashboard URL matches allowed origin
- Use browser dev tools to inspect CORS headers

## Next Steps (Layer 1+)

- Add authentication
- Rate limiting
- Metric aggregation endpoints
- Real-time WebSocket support (Layer 4)
- Caching layer