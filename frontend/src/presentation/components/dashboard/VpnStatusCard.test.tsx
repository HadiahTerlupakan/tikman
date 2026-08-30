import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { WireguardPeer } from "@/domain/entities";
import { VpnStatusCard } from "./VpnStatusCard";

function peer(overrides: Partial<WireguardPeer>): WireguardPeer {
  return {
    id: "p1",
    siteId: "s1",
    name: "Depok",
    tunnelAddress: "10.10.0.2",
    allowedIps: ["192.168.220.0/24"],
    persistentKeepalive: 25,
    enabled: true,
    connected: true,
    lastHandshakeAt: null,
    rxBytes: 0,
    txBytes: 0,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

describe("VpnStatusCard", () => {
  it("reports when no tunnel has been set up", () => {
    render(<VpnStatusCard peers={[]} />);

    expect(screen.getByText("No site tunnels configured.")).toBeInTheDocument();
  });

  it("counts connected sites against the ones expected to be up", () => {
    render(
      <VpnStatusCard
        peers={[
          peer({ id: "1", name: "Depok" }),
          peer({ id: "2", name: "Bekasi", connected: false }),
        ]}
      />,
    );

    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("of 2 sites connected")).toBeInTheDocument();
  });

  it("names the sites that are down, since that is the actionable part", () => {
    render(
      <VpnStatusCard
        peers={[
          peer({ id: "1", name: "Depok", connected: false }),
          peer({ id: "2", name: "Bekasi", connected: false }),
        ]}
      />,
    );

    expect(screen.getByText(/Depok, Bekasi/)).toBeInTheDocument();
  });

  it("leaves switched-off tunnels out of the ratio but still says they exist", () => {
    // Counting a deliberately disabled site as "down" would report a fault the
    // operator created on purpose.
    render(
      <VpnStatusCard
        peers={[
          peer({ id: "1", name: "Depok" }),
          peer({ id: "2", name: "Lama", enabled: false, connected: false }),
        ]}
      />,
    );

    expect(screen.getByText("of 1 sites connected")).toBeInTheDocument();
    expect(screen.getByText("1 switched off, not counted")).toBeInTheDocument();
  });

  it("admits when the status could not be loaded", () => {
    render(<VpnStatusCard peers={undefined} isError />);

    expect(
      screen.getByText("Tunnel status could not be loaded."),
    ).toBeInTheDocument();
  });
});
