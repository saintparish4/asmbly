import { createClient } from "redis";

/**
 *  Redis storage client for metrics
 */
export class MetricsStorage {
  constructor(redisUrl, ttl) {
    this.redisUrl = redisUrl;
    this.ttl = ttl;
    this.client = null;
  }

  /**
   * Connect to redis
   */
  async connect() {
    this.client = createClient({ url: this.redisUrl });

    this.client.on("error", (err) => {
      console.error("Redis connection error:", err);
    });

    await this.client.connect();
  }

  /**
   * Disconnect from redis
   */
  async disconnect() {
    if (this.client) {
      await this.client.quit();
    }
  }

  /**
   * Store health check metrics
   * Key pattern: metrics:{region}:{target}:{timestamp}
   */
  async storeMetric(metric) {
    const key = this._buildKey(metric);
    await this.client.set(key, JSON.stringify(metric), {
      EX: this.ttl,
    });
  }

  /**
   * Get latest metrics for a target
   */
  async getLatestMetrics(region, target, limit = 10) {
    const pattern = `metrics:${region}:${this._sanitizeUrl(target)}:*`;
    const keys = await this.client.keys(pattern);

    // Sort keys by timestamp (descending)
    const sortedKeys = keys.sort().reverse().slice(0, limit);

    const metrics = [];
    for (const key of sortedKeys) {
      const data = await this.client.get(key);
      if (data) {
        metrics.push(JSON.parse(data));
      }
    }

    return metrics;
  }

  /**
   * Get all metrics for a region
   */
  async getRegionMetrics(region, limit = 50) {
    const pattern = `metrics:${region}:*`;
    const keys = await this.client.keys(pattern);

    const sortedKeys = keys.sort().reverse().slice(0, limit);

    const metrics = [];
    for (const key of sortedKeys) {
      const data = await this.client.get(key);
      if (data) {
        metrics.push(JSON.parse(data));
      }
    }

    return metrics;
  }

  /**
   * Build redis key from metric
   */
  _buildKey(metric) {
    const sanitizedTarget = this._sanitizeUrl(metric.target);
    return `metrics:${metric.region}:${sanitizedTarget}:${metric.timestamp}`;
  }
}
