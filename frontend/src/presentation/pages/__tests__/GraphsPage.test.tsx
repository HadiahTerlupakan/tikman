import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphsPage } from "../GraphsPage";
import { OntStatus } from "@/domain/entities/Ont";

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

vi.mock("@/application/hooks/useOnts", () => ({
  useOnts: () => ({
    isLoading: false,
    data: {
      data: [
        makeOnt({ id: "1", serialNumber: "RTEGC609D6CF", name: "Budi Santoso" }),
        makeOnt({ id: "2", serialNumber: "ZTEGC1234567", name: "Warnet Maju" }),
      ],
      total: 2,
    },
  }),
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
  it("filters graph cards by serial number or ONT name", async () => {
    render(<GraphsPage />);

    expect(screen.getByText(/RTEGC609D6CF/)).toBeInTheDocument();
    expect(screen.getByText(/ZTEGC1234567/)).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText("Search serial number or ONT name"), "warnet");

    expect(screen.queryByText(/RTEGC609D6CF/)).not.toBeInTheDocument();
    expect(screen.getByText(/ZTEGC1234567/)).toBeInTheDocument();
    expect(screen.getByText("Showing 1 of 1 ONTs")).toBeInTheDocument();
  });
});
