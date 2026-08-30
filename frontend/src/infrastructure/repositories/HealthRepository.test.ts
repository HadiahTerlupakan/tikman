import { beforeEach, describe, expect, it, vi } from "vitest";
import { HealthRepository } from "./HealthRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("HealthRepository", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("reads the per-dependency status", async () => {
    get.mockResolvedValue({
      data: {
        status: "healthy",
        time: "2026-08-22T02:00:00Z",
        dependencies: { database: "up", redis: "up", worker: "up" },
        workerLastBeat: "2026-08-22T01:59:40Z",
      },
    });

    const health = await new HealthRepository().get();

    expect(get).toHaveBeenCalledWith("/health");
    expect(health.status).toBe("healthy");
    expect(health.dependencies).toEqual({
      database: "up",
      redis: "up",
      worker: "up",
    });
    expect(health.workerLastBeat).toBe("2026-08-22T01:59:40Z");
  });

  it("calls the worker unknown when an older API does not report it", async () => {
    // The API and the app deploy separately; a build without the worker row
    // must not be read as a worker that is down.
    get.mockResolvedValue({
      data: {
        status: "healthy",
        dependencies: { database: "up", redis: "up" },
      },
    });

    const health = await new HealthRepository().get();

    expect(health.dependencies.worker).toBe("unknown");
    expect(health.workerLastBeat).toBeUndefined();
  });

  it("reports degraded status from a 503 body", async () => {
    // The API answers 503 when a dependency is down; axios rejects, but the body
    // still carries which one, so the card can name it instead of going blank.
    get.mockRejectedValue({
      response: {
        status: 503,
        data: {
          status: "degraded",
          dependencies: { database: "down", redis: "up" },
        },
      },
    });

    const health = await new HealthRepository().get();

    expect(health.status).toBe("degraded");
    expect(health.dependencies.database).toBe("down");
  });

  it("reports the API itself as unreachable when there is no response", async () => {
    get.mockRejectedValue(new Error("Network Error"));

    const health = await new HealthRepository().get();

    expect(health.status).toBe("unreachable");
    expect(health.dependencies.database).toBe("unknown");
    expect(health.dependencies.redis).toBe("unknown");
  });

  it("survives a 200 that is not the health payload", async () => {
    // nginx serves the SPA fallback for any unproxied path, so a misrouted
    // /health returns index.html with status 200. Trusting the body here made
    // the dashboard crash on health.dependencies.database.
    get.mockResolvedValue({ data: "<!DOCTYPE html><html></html>" });

    const health = await new HealthRepository().get();

    expect(health.status).toBe("unreachable");
    expect(health.dependencies.database).toBe("unknown");
  });

  it("fills in missing dependency keys rather than exposing undefined", async () => {
    get.mockResolvedValue({ data: { status: "healthy", dependencies: {} } });

    const health = await new HealthRepository().get();

    expect(health.dependencies.database).toBe("unknown");
    expect(health.dependencies.redis).toBe("unknown");
  });
});
