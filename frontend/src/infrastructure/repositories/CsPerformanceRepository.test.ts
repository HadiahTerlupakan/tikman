import { beforeEach, describe, expect, it, vi } from "vitest";
import { CsPerformanceRepository } from "./CsPerformanceRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("CsPerformanceRepository", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("asks for the summary of the dates given and unwraps the envelope", async () => {
    get.mockResolvedValue({ data: { data: { targetMinutes: 15 } } });

    const summary = await new CsPerformanceRepository().getSummary({
      from: "2026-09-01",
      to: "2026-09-15",
    });

    expect(get).toHaveBeenCalledWith("/api/v1/cs/performance/summary", {
      params: { from: "2026-09-01", to: "2026-09-15" },
    });
    expect(summary).toEqual({ targetMinutes: 15 });
  });

  // The client decamelizes request bodies, never query params, so user_id has
  // to be spelled the way the server reads it.
  it("sends the wait query in snake_case and keeps the total", async () => {
    get.mockResolvedValue({ data: { data: [{ id: "w1" }], total: 7 } });

    const page = await new CsPerformanceRepository().getWaits({
      from: "2026-09-01",
      to: "2026-09-15",
      userId: "u1",
      limit: 20,
      offset: 40,
    });

    expect(get).toHaveBeenCalledWith("/api/v1/cs/performance/waits", {
      params: {
        from: "2026-09-01",
        to: "2026-09-15",
        user_id: "u1",
        limit: 20,
        offset: 40,
      },
    });
    expect(page).toEqual({ items: [{ id: "w1" }], total: 7 });
  });
});
