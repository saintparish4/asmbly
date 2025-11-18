import request from "supertest";
import express from "express";
import { createRouter } from "../src/routes.js";

// Mock Redis client
const mockRedisClient = {
  keys: jest.fn(),
  get: jest.fn(),
};

const app = express();
app.use(express.json());
app.use("/api", createRouter(mockRedisClient));

describe("API Endpoint Tests", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("GET /api/health", () => {
    test("should return health status", async () => {
      const response = await request(app).get("/api/health").expect(200);

      expect(response.body).toHaveProperty("status", "ok");
      expect(response.body).toHaveProperty("timestamp");
    });
  });

  describe("GET /api/metrics/target", () => {
    test("should return target metrics", async () => {
      const mockMetric = {
        region: "us-east-1",
        target: "https://example.com",
        latency_ms: 45,
        status: 200,
        timestamp: "2024-01-15T10:30:00Z",
        error: null,
      };

      mockRedisClient.keys.mockResolvedValue([
        "metrics:us-east-1:https___example_com:2024",
      ]);
      mockRedisClient.get.mockResolvedValue(JSON.stringify(mockMetric));

      const response = await request(app)
        .get("/api/metrics/target?region=us-east-1&url=https://example.com")
        .expect(200);

      expect(response.body).toHaveProperty("region", "us-east-1");
      expect(response.body).toHaveProperty("target", "https://example.com");
      expect(response.body).toHaveProperty("count", 1);
      expect(response.body.metrics).toHaveLength(1);
      expect(response.body.metrics[0]).toMatchObject(mockMetric);
    });

    test("should return 400 if region missing", async () => {
      const response = await request(app)
        .get("/api/metrics/target?url=https://example.com")
        .expect(400);

      expect(response.body).toHaveProperty("error");
    });

    test("should return 400 if url missing", async () => {
      const response = await request(app)
        .get("/api/metrics/target?region=us-east-1")
        .expect(400);

      expect(response.body).toHaveProperty("error");
    });
  });

  describe("GET /api/metrics/region", () => {
    test("should return region metrics grouped by target", async () => {
      const mockMetrics = [
        {
          region: "us-east-1",
          target: "https://example1.com",
          latency_ms: 45,
          status: 200,
          timestamp: "2024-01-15T10:30:00Z",
          error: null,
        },
        {
          region: "us-east-1",
          target: "https://example2.com",
          latency_ms: 60,
          status: 200,
          timestamp: "2024-01-15T10:30:00Z",
          error: null,
        },
      ];

      mockRedisClient.keys.mockResolvedValue([
        "metrics:us-east-1:example1:2024",
        "metrics:us-east-1:example2:2024",
      ]);
      mockRedisClient.get
        .mockResolvedValueOnce(JSON.stringify(mockMetrics[0]))
        .mockResolvedValueOnce(JSON.stringify(mockMetrics[1]));

      const response = await request(app)
        .get("/api/metrics/region?region=us-east-1")
        .expect(200);

      expect(response.body).toHaveProperty("region", "us-east-1");
      expect(response.body).toHaveProperty("count", 2);
      expect(response.body).toHaveProperty("targets", 2);
      expect(response.body.metrics).toHaveProperty("https://example1.com");
      expect(response.body.metrics).toHaveProperty("https://example2.com");
    });

    test("should return 400 if region missing", async () => {
      const response = await request(app)
        .get("/api/metrics/region")
        .expect(400);

      expect(response.body).toHaveProperty("error");
    });
  });

  describe("GET /api/metrics/latest", () => {
    test("should return only latest metric per target", async () => {
      const mockMetrics = [
        {
          region: "us-east-1",
          target: "https://example.com",
          latency_ms: 45,
          status: 200,
          timestamp: "2024-01-15T10:30:00Z",
          error: null,
        },
        {
          region: "us-east-1",
          target: "https://example.com",
          latency_ms: 50,
          status: 200,
          timestamp: "2024-01-15T10:31:00Z",
          error: null,
        },
      ];

      mockRedisClient.keys.mockResolvedValue([
        "metrics:us-east-1:example:2024-1",
        "metrics:us-east-1:example:2024-2",
      ]);
      mockRedisClient.get
        .mockResolvedValueOnce(JSON.stringify(mockMetrics[0]))
        .mockResolvedValueOnce(JSON.stringify(mockMetrics[1]));

      const response = await request(app)
        .get("/api/metrics/latest?region=us-east-1")
        .expect(200);

      expect(response.body).toHaveProperty("count", 1);
      expect(response.body.metrics).toHaveLength(1);
      expect(response.body.metrics[0].timestamp).toBe("2024-01-15T10:31:00Z");
    });

    test("should return 400 if region missing", async () => {
      const response = await request(app)
        .get("/api/metrics/latest")
        .expect(400);

      expect(response.body).toHaveProperty("error");
    });
  });
});
