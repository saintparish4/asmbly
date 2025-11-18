/**
 * Perform HTTP health check on a target URL
 */
export async function performHealthCheck(target, region) {
  const startTime = Date.now();
  const timestamp = new Date().toISOString();

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), target.timeout);

    const response = await fetch(target.url, {
      method: "GET",
      signal: controller.signal,
      headers: {
        "User-Agent": "Asbmly=HealthCheck/1.0",
      },
    });

    clearTimeout(timeoutId);

    const latency = Date.now() - startTime;

    return {
      region,
      target: target.url,
      latency_ms: latency,
      status: response.status,
      timestamp,
      error: null,
    };
  } catch (error) {
    const latency = Date.now() - startTime;

    return {
      region,
      target: target.url,
      latency_ms: latency,
      status: 0,
      timestamp,
      error: error.name === "AbortError" ? "Timeout" : error.message,
    };
  }
}

/**
 * Validate response body (optional check)
 */
export function validateResponse(body, expectedStatus) {
  if (!expectedContent) {
    return true;
  }

  return body.includes(expectedContent);
}

/**
 * Determine if health check passed
 */
export function isHealthy(metric) {
  return metric.status === 200 && metric.error === null;
}
