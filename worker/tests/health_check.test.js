import {
  performHealthCheck,
  validateResponse,
  isHealthy,
} from "../src/health_check";

describe("Health Check Tests", () => {
  describe("performHealthCheck", () => {
    test("should return successful metric for valid URL", async () => {
      const target = {
        url: "https://www.example.com",
        timeout: 5000,
      };

      const metric = await performHealthCheck(target, "us-east-1");

      expect(metric).toHaveProperty("region", "us-east-1");
      expect(metric).toHaveProperty("target", "https://www.example.com");
      expect(metric).toHaveProperty("latency_ms");
      expect(metric).toHaveProperty("status");
      expect(metric).toHaveProperty("timestamp");
      expect(metric).toHaveProperty("error");

      expect(typeof metric.latency_ms).toBe("number");
      expect(metric.latency_ms).toBeGreaterThan(0);
    }, 10000);

    test("should handle timeout", async () => {
      const target = {
        url: "https://httpstat.us/200?sleep=10000",
        timeout: 100,
      };

      const metric = await performHealthCheck(target, "us-east-1");

      expect(metric.error).toBe("Timeout");
      expect(metric.status).toBe(0);
    }, 15000);

    test("should handle DNS failure", async () => {
      const target = {
        url: "https://this-domain-does-not-exist-12345.com",
        timeout: 5000,
      };

      const metric = await performHealthCheck(target, "us-east-1");

      expect(metric.error).toBeTruthy();
      expect(metric.status).toBe(0);
    }, 10000);
  });

  describe("validateResponse", () => {
    test("should return true when no expected content provided", () => {
      const result = validateResponse("any content", null);
      expect(result).toBe(true);
    });

    test("should return true when content matches", () => {
      const result = validateResponse("Hello World", "World");
      expect(result).toBe(true);
    });

    test("should return false when content does not match", () => {
      const result = validateResponse("Hello World", "Goodbye");
      expect(result).toBe(false);
    });
  });

  describe("isHealthy", () => {
    test("should return true for healthy metric", () => {
      const metric = {
        status: 200,
        error: null,
      };

      expect(isHealthy(metric)).toBe(true);
    });

    test("should return false for non-200 status", () => {
      const metric = {
        status: 500,
        error: null,
      };

      expect(isHealthy(metric)).toBe(false);
    });

    test("should return false when error exists", () => {
      const metric = {
        status: 200,
        error: "Timeout",
      };

      expect(isHealthy(metric)).toBe(false);
    });
  });
});
