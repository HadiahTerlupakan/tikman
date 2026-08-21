import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { GraphsPage } from "../GraphsPage";
import { OntStatus } from "@/domain/entities/Ont";

const getTrafficTimeSeries = vi.fn().mockResolvedValue([]);

vi.mock("@/application/hooks/useOlts", () => ({
  useOlts: () => ({ data: [{ id: "olt-1", name: "OLT Cariu" }] }),
}));

vi.mock("@/application/hooks/useOnts", () => ({
  useOnts: () => ({
    isLoading: false,
    data: {
      data: [makeOnt({ id: "1", status: OntStatus.ONLINE }), makeOnt({ id: "2", status: OntStatus.OFFLINE })],
      total: 2,
    },
  }),
}));

vi.mock("@/application/hooks/useOntMetrics", () => ({
  useOntTrafficTimeSeries: () => ({ isLoading: false, data: [] }),
}));

function makeOnt(overrides: Partial<Record<string, unknown>>) {
  return {
    id: "ont-1",
    oltId: "olt-1",
    oltName: "OLT1",
    slot: 1,
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

describe("GraphsPage date range", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getTrafficTimeSeries.mockResolvedValue([]);
  });

  it("displays ONT status tag on each traffic card", () => {
    render(<GraphsPage />);

    expect(screen.getByText("online")).toBeInTheDocument();
    expect(screen.getByText("offline")).toBeInTheDocument();
  });

  it("renders date range controls", () => {
    render(<GraphsPage />);

    expect(screen.getByPlaceholderText("Start date")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("End date")).toBeInTheDocument();
  });
});
