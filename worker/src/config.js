import { readFileSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Load monitoring targets from configuration file
 */
export function loadTargets() {
  const targetsPath = join(__dirname, "../shared/targets.json");
  const data = readFileSync(targetsPath, "utf8");
  return JSON.parse(data).targets;
}

/**
 * Get worker configuration from environment
 */
export function getConfig() {
  return {
    region: process.env.WORKER_REGION || "us-east-1",
    redisUrl: process.env.REDIS_URL || "redis://localhost:6379",
    redisTTL: parseInt(process.env.REDIS_TTL || "86400", 10),
    checkInterval: parseInt(process.env.CHECK_INTERVAL || "60000", 10),
  };
}
