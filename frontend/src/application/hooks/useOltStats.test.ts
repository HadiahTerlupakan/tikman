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

type StatsQueryOptions = {
  refetchInterval: (query: {
    state: { data?: { totalOnts: number; phase?: string } };
  }) => number;
};

function useStatsQueryOptions(): StatsQueryOptions {
  useOltStats("olt-1");
  const results = vi.mocked(useQuery).mock.results;
  return results[results.length - 1]?.value as StatsQueryOptions;
}

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
    const options = useStatsQueryOptions();

    expect(options.refetchInterval({ state: { data: { totalOnts: 0 } } })).toBe(
      5000,
    );
  });

  it("slows to one minute once discovery has finished", () => {
    const options = useStatsQueryOptions();

    expect(
      options.refetchInterval({
        state: { data: { totalOnts: 197, phase: "completed" } },
      }),
    ).toBe(60000);
  });

  // Registration lands one PON port at a time, so the count keeps climbing well
  // after the first ONTs appear. Slowing down there froze the progress bar for
  // a minute between instalments.
  it("keeps the fast interval while ONTs are still being registered", () => {
    const options = useStatsQueryOptions();

    for (const phase of ["discovering", "polling"]) {
      expect(
        options.refetchInterval({ state: { data: { totalOnts: 25, phase } } }),
      ).toBe(5000);
    }
  });
});
