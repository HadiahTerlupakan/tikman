import { describe, expect, it, vi } from "vitest";
import { useQuery } from "@tanstack/react-query";
import { useOntTrafficTimeSeries } from "./useOntMetrics";

vi.mock("@tanstack/react-query", () => ({
  useQuery: vi.fn((options) => options),
}));

vi.mock("@/infrastructure/repositories", () => ({
  OntRepository: class {
    getTrafficTimeSeries = vi.fn();
  },
}));

describe("useOntTrafficTimeSeries", () => {
  it("polls in the background on the worker cadence", () => {
    useOntTrafficTimeSeries("ont-1", "3h");

    expect(useQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        refetchInterval: 60000,
        refetchIntervalInBackground: true,
      }),
    );
  });
});
