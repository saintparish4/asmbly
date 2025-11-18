import dotenv from "dotenv";
import { performHealthCheck } from "./health_check";
import { MetricsStorage } from "./storage";
import { loadTargets, getConfig } from "./config";

// Load environment vars
dotenv.config();

/**
 * Main worker function
 */
async function runWorker() {
  const config = getConfig();
  const targets = loadTargets();

  console.log(`Starting health check worker in region: ${config.region}`);
  console.log(`Monitoring ${targets.length} targets`);

  const storage = new MetricsStorage(config.redisUrl, config.redisTTL);

  try {
    await storage.connect();
    console.log("Connected to Redis");

    // Run checks indefinitely
    while (true) {
      await runHealthChecks(targets, config.region, storage);
      await sleep(config.checkInterval);
    }
  } catch (error) {
    console.error("Worker error:", error);
    await storage.disconnect();
    process.exit(1);
  }
}

/**
 * Run health checks for all targets
 */
async function runHealthChecks(targets, region, storage) {
  console.log(`Running health checks at ${new Date().toISOString()}`);

  const checks = targets.map(async (target) => {
    try {
      const metric = await performHealthCheck(target, region);
      await storage.storeMetric(metric);

      const status = metric.error ? "❌" : "✅";
      console.log(
        `${status} ${target.url}: ${metric.latency_ms}ms (${metric.status})`
      );
    } catch (error) {
      console.error(`Error checking ${target.url}:`, error.message);
    }
  });

  await Promise.all(checks);
}

/**
 * Sleep for specified milliseconds
 */
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// Start the worker
if (import.meta.url === `file://${process.argv[1]}`) {
  runWorker().catch(console.error);
}

export { runWorker, runHealthChecks };
