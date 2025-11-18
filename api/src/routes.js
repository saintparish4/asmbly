import express from "express";
import { getTargetMetrics, getRegionMetrics } from "./redis.js";

export function createRouter(redisClient) {
  const router = express.Router();

  /**
   * GET /health
   * Health check endpoint
   */
  router.get("/health", (req, res) => {
    res.json({ status: "ok", timestamp: new Date().toISOString() });
  });

  /**
   * GET /metrics/target
   * Get latest metrics for a specific target
   * Query params: region, url, limit
   */
  router.get("/metrics/target", async (req, res) => {
    try {
      const { region, url, limit } = req.query;

      if (!region || !url) {
        return res.status(400).json({
          error: "Missing required parameters: region and url",
        });
      }

      const metrics = await getTargetMetrics(
        redisClient,
        region,
        url,
        parseInt(limit || "10", 10)
      );

      res.json({
        region,
        target: url,
        count: metrics.length,
        metrics,
      });
    } catch (error) {
      console.error("Error fetching target metrics:", error);
      res.status(500).json({ error: "Internal server error" });
    }
  });

  /**
   * GET /metrics/region
   * Get all metrics for a region
   * Query params: region, limit
   */
  router.get("/metrics/region", async (req, res) => {
    try {
      const { region, limit } = req.query;

      if (!region) {
        return res.status(400).json({
          error: "Missing required parameter: region",
        });
      }

      const metrics = await getRegionMetrics(
        redisClient,
        region,
        parseInt(limit || "50", 10)
      );

      // Group metrics by target
      const grouped = metrics.reduce((acc, metric) => {
        if (!acc[metric.target]) {
          acc[metric.target] = [];
        }
        acc[metric.target].push(metric);
        return acc;
      }, {});

      res.json({
        region,
        count: metrics.length,
        targets: Object.keys(grouped).length,
        metrics: grouped,
      });
    } catch (error) {
      console.error("Error fetching region metrics:", error);
      res.status(500).json({ error: "Internal server error" });
    }
  });

  /**
   * GET /metrics/latest
   * Get latest metric for each target in a region
   * Query params: region
   */
  router.get("/metrics/latest", async (req, res) => {
    try {
      const { region } = req.query;

      if (!region) {
        return res.status(400).json({
          error: "Missing required parameter: region",
        });
      }

      const allMetrics = await getRegionMetrics(redisClient, region, 100);

      // Get only the latest metric per target
      const latest = {};
      allMetrics.forEach((metric) => {
        if (
          !latest[metric.target] ||
          new Date(metric.timestamp) > new Date(latest[metric.target].timestamp)
        ) {
          latest[metric.target] = metric;
        }
      });

      res.json({
        region,
        count: Object.keys(latest).length,
        metrics: Object.values(latest),
      });
    } catch (error) {
      console.error("Error fetching latest metrics:", error);
      res.status(500).json({ error: "Internal server error" });
    }
  });

  return router;
}
