import { createClient } from "redis";

let client = null;

/**
 * Get or create Redis client
 */
export async function getRedisClient(redisUrl) {
  if (client && client.isOpen) {
    return client;
  }

  client = createClient({ url: redisUrl });

  client.on("error", (err) => {
    console.error("Redis Client Error:", err);
  });

  await client.connect();
  return client;
}

/**
 * Get latest metrics for a specific target
 */
export async function getTargetMetrics(
  redisClient,
  region,
  target,
  limit = 10
) {
  const sanitizedTarget = sanitizeUrl(target);
  const pattern = `metrics:${region}:${sanitizedTarget}:*`;

  const keys = await redisClient.keys(pattern);
  const sortedKeys = keys.sort().reverse().slice(0, limit);

  const metrics = [];
  for (const key of sortedKeys) {
    const data = await redisClient.get(key);
    if (data) {
      metrics.push(JSON.parse(data));
    }
  }

  return metrics;
}

/**
 * Get all metrics for a region
 */
export async function getRegionMetrics(redisClient, region, limit = 50) {
  const pattern = `metrics:${region}:*`;

  const keys = await redisClient.keys(pattern);
  const sortedKeys = keys.sort().reverse().slice(0, limit);

  const metrics = [];
  for (const key of sortedKeys) {
    const data = await redisClient.get(key);
    if (data) {
      metrics.push(JSON.parse(data));
    }
  }

  return metrics;
}

/**
 * Sanitize URL for Redis key lookup
 */
function sanitizeUrl(url) {
  return url.replace(/[^a-zA-Z0-9]/g, "_");
}

/**
 * Close Redis connection
 */
export async function closeRedisClient() {
  if (client) {
    await client.quit();
    client = null;
  }
}
