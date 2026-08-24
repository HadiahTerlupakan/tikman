import { describe, expect, it, vi } from "vitest";
import { useQuery } from "@tanstack/react-query";
import { useOltStats } from "./useOlts";

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: vi.fn((options) => options),
  };
});

vi.mock("@/infrastructure/repositories", () => ({
  OltRepository: class {
    getStats = vi.fn();
  },
}));

describe("useOltStats", () => {
  it("refreshes OLT stats in the background", () => {
    useOltStats("olt-1");

    expect(useQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["olts", "olt-1", "stats"],
        enabled: true,
        refetchIntervalInBackground: true,
      }),
    );
  });

  it("polls every five seconds while discovery has no ONTs", () => {
    useOltStats("olt-1");
    const results = vi.mocked(useQuery).mock.results;
    const options = results[results.length - 1]?.value as {
      refetchInterval: (query: {
        state: { data?: { totalOnts: number } };
      }) => number;
    };

    expect(options.refetchInterval({ state: { data: { totalOnts: 0 } } })).toBe(
      5000,
    );
  });

  it("slows to one minute after ONTs are available", () => {
    useOltStats("olt-1");
    const results = vi.mocked(useQuery).mock.results;
    const options = results[results.length - 1]?.value as {
      refetchInterval: (query: {
        state: { data?: { totalOnts: number } };
      }) => number;
    };

    expect(
      options.refetchInterval({ state: { data: { totalOnts: 197 } } }),
    ).toBe(60000);
  });
});
