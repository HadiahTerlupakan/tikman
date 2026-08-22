import { describe, expect, it, vi } from "vitest";
import { useQuery } from "@tanstack/react-query";
import { useOlts } from "./useOlts";

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: vi.fn((options) => options),
  };
});

vi.mock("@/infrastructure/repositories", () => ({
  OltRepository: class {
    getAll = vi.fn();
    getBySite = vi.fn();
    getStats = vi.fn();
  },
}));

describe("useOlts", () => {
  it("polls OLT list in the background", () => {
    useOlts();

    expect(useQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["olts"],
        refetchInterval: 60000,
        refetchIntervalInBackground: true,
      }),
    );
  });
});
