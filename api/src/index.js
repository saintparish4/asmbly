import dotenv from "dotenv";
import express from "express";
import cors from "cors";
import { getRedisClient, closeRedisClient } from "./redis.js";
import { createRouter } from "./routes.js";

// Load environment variables
dotenv.config();

const PORT = process.env.API_PORT || 3000;
const REDIS_URL = process.env.REDIS_URL || "redis://localhost:6379";
const CORS_ORIGIN = process.env.CORS_ORIGIN || "*";

/**
 * Start API server
 */
async function startServer() {
  const app = express();

  // Middleware
  app.use(cors({ origin: CORS_ORIGIN }));
  app.use(express.json());

  // Request logging
  app.use((req, res, next) => {
    console.log(`${new Date().toISOString()} ${req.method} ${req.path}`);
    next();
  });

  try {
    // Connect to Redis
    const redisClient = await getRedisClient(REDIS_URL);
    console.log("Connected to Redis");

    // Setup routes
    const router = createRouter(redisClient);
    app.use("/api", router);

    // 404 handler
    app.use((req, res) => {
      res.status(404).json({ error: "Not found" });
    });

    // Error handler
    app.use((err, req, res, next) => {
      console.error("Unhandled error:", err);
      res.status(500).json({ error: "Internal server error" });
    });

    // Start server
    const server = app.listen(PORT, () => {
      console.log(`API server running on port ${PORT}`);
      console.log(`CORS origin: ${CORS_ORIGIN}`);
    });

    // Graceful shutdown
    process.on("SIGTERM", async () => {
      console.log("SIGTERM received, shutting down gracefully");
      server.close(() => {
        console.log("HTTP server closed");
      });
      await closeRedisClient();
      process.exit(0);
    });

    process.on("SIGINT", async () => {
      console.log("SIGINT received, shutting down gracefully");
      server.close(() => {
        console.log("HTTP server closed");
      });
      await closeRedisClient();
      process.exit(0);
    });

    return server;
  } catch (error) {
    console.error("Failed to start server:", error);
    process.exit(1);
  }
}

// Start server if run directly
if (import.meta.url === `file://${process.argv[1]}`) {
  startServer();
}

export { startServer };
