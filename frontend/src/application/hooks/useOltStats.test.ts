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
        refetchInterval: 60000,
        refetchIntervalInBackground: true,
      }),
    );
  });
});
