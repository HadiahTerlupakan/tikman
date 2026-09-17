import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Odp, Ont } from "@/domain/entities";
import { OntOdpPanel } from "./OntOdpPanel";

const odp = {
  id: "odp-1",
  code: "ODP-01",
  portCount: 8,
  usedPorts: 5,
  address: "",
  notes: "",
  routeMeters: 0,
} as Odp;

// A freshly-placed box: the new map page makes capacity optional when
// creating a node, so this — not a box with a stated size — is the ODP an
// operator in the field is most likely to open first.
const odpUnlimited = {
  id: "odp-2",
  code: "ODP-UNLIMITED",
  portCount: 0,
  usedPorts: 0,
  address: "",
  notes: "",
  routeMeters: 0,
} as Odp;

const ont = { id: "ont-1", serialNumber: "ZTEGC0000001" } as Ont;

const subscribersByOdp: Record<string, Ont[]> = {
  "odp-1": [],
  "odp-2": [{ id: "ont-77", serialNumber: "ZTEGC0000077", odpPort: 5 } as Ont],
};

vi.mock("@/application/hooks", () => ({
  useOdps: () => ({ data: [odp, odpUnlimited] }),
  useOdpSubscribers: (odpId?: string) => ({
    data: odpId ? subscribersByOdp[odpId] : undefined,
  }),
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
      await screen.findByTitle("ODP-01 (kapasitas 8)"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/5\/8/)).not.toBeInTheDocument();
  });

  // freePorts(0, …) loops `for (port = 1; port <= 0)`, which never runs — an
  // ODP placed from the map with capacity left blank (the design's own
  // two-tap path) read as full with no port to pick, even though the rest of
  // the system, including the backend, treats capacity 0 as unlimited.
  it("lets a port be entered by number on a box with no stated capacity, instead of reading as full", async () => {
    render(<OntOdpPanel ont={ont} />);

    await userEvent.click(screen.getAllByRole("combobox")[0]);
    await userEvent.click(await screen.findByTitle("ODP-UNLIMITED"));

    expect(screen.queryByText(/sudah penuh/)).not.toBeInTheDocument();
    // The occupant of port 5 is real (useOdpSubscribers), unlike usedPorts.
    expect(await screen.findByText(/5 \(ZTEGC0000077\)/)).toBeInTheDocument();

    const portInput = screen.getByPlaceholderText("Nomor port");
    await userEvent.type(portInput, "999");
    expect(portInput).toHaveValue("999");
  });
});
