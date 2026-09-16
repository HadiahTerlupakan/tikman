import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Odp, Ont } from "@/domain/entities";
import { OntOdpPanel } from "./OntOdpPanel";

const odp = {
  id: "odp-1",
  code: "ODP-CARIU-01",
  portCount: 8,
  usedPorts: 5,
  address: "",
  notes: "",
  routeMeters: 0,
} as Odp;

const ont = { id: "ont-1", serialNumber: "ZTEGC0000001" } as Ont;

vi.mock("@/application/hooks", () => ({
  useOdps: () => ({ data: [odp] }),
  useOdpSubscribers: () => ({ data: [] }),
  useAssignOntToOdp: () => ({
    mutate: vi.fn(),
    isPending: false,
    isError: false,
  }),
}));

describe("OntOdpPanel", () => {
  // usedPorts on the mock is 5, so a fabricated ratio would read "(5/8)"
  // here — a plausible-looking number nothing actually computes for a
  // mapping node (see toOdp in DistributionRepository). Only the stated
  // capacity may reach the label.
  it("shows the box's capacity, never a fabricated occupancy ratio", async () => {
    render(<OntOdpPanel ont={ont} />);

    await userEvent.click(screen.getAllByRole("combobox")[0]);

    expect(
      await screen.findByTitle("ODP-CARIU-01 (kapasitas 8)"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/5\/8/)).not.toBeInTheDocument();
  });
});
