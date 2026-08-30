import { describe, it, expect, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphsPage } from "../GraphsPage";
import { OntStatus } from "@/domain/entities/Ont";
import { SEARCH_DEBOUNCE_MS } from "@/shared/config/limits";

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    Select: () => null,
  };
});

vi.mock("@/application/hooks/useOlts", () => ({
  useOlts: () => ({ data: [{ id: "olt-1", name: "OLT Cariu" }] }),
}));

// The page no longer filters what it was sent; it asks the database. What the
// test can observe, and what matters, is the query it asks with.
const ontQueries: Array<Record<string, unknown>> = [];

vi.mock("@/application/hooks/useOnts", () => ({
  useOnts: (params: Record<string, unknown>) => {
    ontQueries.push(params);
    return {
      isLoading: false,
      data: {
        data: [
          makeOnt({
            id: "1",
            serialNumber: "RTEGC609D6CF",
            name: "Budi Santoso",
          }),
          makeOnt({
            id: "2",
            serialNumber: "ZTEGC1234567",
            name: "Warnet Maju",
          }),
        ],
        total: 2,
      },
    };
  },
}));

vi.mock("@/application/hooks/useOntMetrics", () => ({
  useOntTrafficTimeSeries: () => ({ isLoading: false, data: [] }),
}));

function makeOnt(overrides: Record<string, unknown>) {
  return {
    id: "ont-1",
    oltId: "olt-1",
    oltName: "OLT1",
    portId: 1,
    ontId: 1,
    serialNumber: "RTEGC609D6CF",
    name: "Budi Santoso",
    description: "",
    status: OntStatus.ONLINE,
    lastSeenAt: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe("GraphsPage", () => {
  it("renders a card for every ONT the server returned", () => {
    ontQueries.length = 0;
    render(<GraphsPage />);

    expect(screen.getByText(/RTEGC609D6CF/)).toBeInTheDocument();
    expect(screen.getByText(/ZTEGC1234567/)).toBeInTheDocument();
    expect(screen.getByText("Showing 2 of 2 ONTs")).toBeInTheDocument();
  });

  it("asks the server for the search rather than filtering what it holds", async () => {
    // Filtering the rows one request returned could only ever search that page.
    // On an OLT larger than a page it left ONTs the search could not reach.
    vi.useFakeTimers();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    ontQueries.length = 0;

    render(<GraphsPage />);
    await user.type(
      screen.getByPlaceholderText("Search serial number or ONT name"),
      "warnet",
    );
    await act(async () => {
      vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
    });

    expect(ontQueries[ontQueries.length - 1].search).toBe("warnet");
    vi.useRealTimers();
  });

  it("asks for one page rather than the whole network", () => {
    ontQueries.length = 0;
    render(<GraphsPage />);

    const query = ontQueries[ontQueries.length - 1];
    expect(query.limit).toBe(9);
    expect(query.offset).toBe(0);
  });
});
