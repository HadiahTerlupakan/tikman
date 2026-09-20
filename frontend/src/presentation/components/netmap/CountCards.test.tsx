import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MappingNode, Olt } from "@/domain/entities";
import { CountCards } from "./CountCards";

// Minimal OLT fixtures — only the fields CountCards actually reads,
// following the same partial-plus-cast fixture OltTable.test.tsx uses.
function olt(overrides: Partial<Olt>): Olt {
  return { id: "olt-1", name: "OLT-01", ...overrides } as Olt;
}

const odpNode: MappingNode = {
  nodeId: "ODP-01",
  type: "odp",
  name: "ODP Satu",
  latitude: -6.2,
  longitude: 106.8,
  capacity: 8,
  splitter: "",
  pppoe: "",
  serialNumber: "",
  notes: "",
};

describe("CountCards", () => {
  // The point of this card: a map that mirrors only some OLTs must not read
  // as complete. Three of six missing on day one is the real production case.
  it("counts OLTs with a position on the map separately from the total fleet", () => {
    const olts = [
      olt({ id: "1", latitude: -6.2, longitude: 106.8 }),
      olt({ id: "2", latitude: -6.3, longitude: 106.9 }),
      olt({ id: "3" }),
    ];
    render(<CountCards nodes={[]} olts={olts} />);

    expect(screen.getByText("2 dari 3 terpasang")).toBeInTheDocument();
  });

  it("reads zero of zero when there are no OLTs at all, rather than crashing", () => {
    render(<CountCards nodes={[]} olts={[]} />);

    expect(screen.getByText("0 dari 0 terpasang")).toBeInTheDocument();
  });

  // An OLT with only one of the two coordinates set is not on the map —
  // syncOLTMapNode (backend) requires both before it mirrors anything.
  it("does not count an OLT missing just one of its two coordinates as placed", () => {
    const olts = [olt({ id: "1", latitude: -6.2 })];
    render(<CountCards nodes={[]} olts={olts} />);

    expect(screen.getByText("0 dari 1 terpasang")).toBeInTheDocument();
  });

  // Regression guard: the existing per-kind breakdown must survive the new
  // card sitting alongside it.
  it("still counts mapped nodes by kind", () => {
    render(<CountCards nodes={[odpNode]} olts={[]} />);

    expect(screen.getByTestId("count-odp")).toHaveTextContent("1");
    expect(screen.getByTestId("count-ont")).toHaveTextContent("0");
  });
});
