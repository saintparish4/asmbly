# CDN Health Check Worker

Single-region health monitoring worker that performs HTTP checks on configured targets and stores metrics in Redis.

## Features

- HTTP health checks with configurable timeouts
- Latency measurement
- Status code validation
- Error handling (timeouts, DNS failures)
- Metrics storage in Redis with TTL
- Scheduled execution with configurable intervals

## Architecture

```
┌─────────────┐
│   Worker    │
│  (Node.js)  │
└──────┬──────┘
       │
       ├──► HTTP Checks (targets)
       │
       └──► Redis (metrics storage)
```

## Setup

### Prerequisites

- Node.js 18+
- Redis (local or remote)

### Installation

```bash
cd worker
npm install
```

### Configuration

Copy the root `.env.example` and configure:

```bash
cp ../.env.example ../.env
```

Environment variables:
- `WORKER_REGION` - Region identifier (e.g., us-east-1)
- `REDIS_URL` - Redis connection string
- `REDIS_TTL` - Metrics TTL in seconds (default: 86400 = 24h)
- `CHECK_INTERVAL` - Check frequency in milliseconds (default: 60000 = 1min)

### Monitoring Targets

Edit `../shared/targets.json`:

```json
{
  "targets": [
    {
      "url": "https://cdn.example.com",
      "interval": 60,
      "timeout": 5000
    }
  ]
}
```

## Running

### Start Redis (using Docker)

```bash
cd ..
docker-compose up -d
```

### Start Worker

```bash
npm start
```

Output:
```
Starting health check worker in region: us-east-1
Monitoring 3 targets
Connected to Redis
Running health checks at 2024-01-15T10:30:00Z
✅ https://www.cloudflare.com: 145ms (200)
✅ https://www.fastly.com: 203ms (200)
❌ https://www.akamai.com: 5001ms (0)
```

## Testing

```bash
npm test
```

Coverage targets: 60-70% of core functionality

## Metrics Schema

Stored in Redis with key pattern: `metrics:{region}:{target}:{timestamp}`

```json
{
  "region": "us-east-1",
  "target": "https://cdn.example.com",
  "latency_ms": 45,
  "status": 200,
  "timestamp": "2024-01-15T10:30:00.000Z",
  "error": null
}
```

## Module Overview

### `src/config.js`
- Load targets from JSON
- Environment configuration

### `src/storage.js`
- Redis client wrapper
- Metric storage operations
- Key management with TTL

### `src/healthCheck.js`
- HTTP health check execution
- Latency measurement
- Error handling

### `src/index.js`
- Main worker loop
- Scheduled execution
- Orchestration

## Deployment

### Local Development
```bash
npm start
```

### Cloudflare Workers (Future)
See Layer 1 for multi-region deployment

### Docker
```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY . .
CMD ["node", "src/index.js"]
```

## Troubleshooting

### Redis Connection Issues
- Verify Redis is running: `docker ps`
- Check connection string in `.env`
- Test connection: `redis-cli ping`

### Timeout Errors
- Increase `timeout` value in targets.json
- Check network connectivity
- Verify target URL is accessible

### High Latency
- Target server may be slow
- Network issues between worker and target
- Check from different region in Layer 1

## Next Steps (Layer 1)

- Deploy to multiple regions
- Worker coordination via Redis Pub/Sub
- Scheduled cron execution
- Target management improvements