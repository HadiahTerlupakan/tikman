import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Olt, WireguardPeer } from "@/domain/entities";
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
    lastHandshakeAt: new Date().toISOString(),
    rxBytes: 0,
    txBytes: 0,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

const olt = (ipAddress: string): Olt => ({ ipAddress }) as Olt;

describe("VpnStatusCard", () => {
  it("reports when no tunnel has been set up", () => {
    render(<VpnStatusCard peers={[]} olts={[]} />);

    expect(screen.getByText("No site tunnels configured.")).toBeInTheDocument();
  });

  it("counts connected sites against the ones expected to be up", () => {
    render(
      <VpnStatusCard
        olts={[]}
        peers={[
          peer({ id: "1", name: "Depok" }),
          peer({ id: "2", name: "Bekasi", connected: false }),
        ]}
      />,
    );

    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("of 2 sites connected")).toBeInTheDocument();
  });

  it("gives each site the evidence behind its state", () => {
    // Without this the card says "down" and leaves the operator guessing
    // whether to wait, call the site, or go paste a config.
    render(
      <VpnStatusCard
        olts={[]}
        peers={[
          peer({
            id: "1",
            name: "Dropped",
            connected: false,
            lastHandshakeAt: new Date(Date.now() - 7_200_000).toISOString(),
          }),
          peer({
            id: "2",
            name: "New",
            connected: false,
            lastHandshakeAt: null,
          }),
        ]}
      />,
    );

    expect(screen.getByText("last seen 2h ago")).toBeInTheDocument();
    expect(screen.getByText("never connected")).toBeInTheDocument();
  });

  it("says how much hardware a broken tunnel is cutting off", () => {
    render(
      <VpnStatusCard
        olts={[olt("192.168.220.22"), olt("192.168.220.23")]}
        peers={[peer({ id: "1", name: "Depok", connected: false })]}
      />,
    );

    expect(screen.getByText("2 OLTs")).toBeInTheDocument();
  });

  it("stays silent about hardware when the tunnel reaches none", () => {
    // A "0 OLTs" badge would read as a fault rather than as "this outage
    // reaches nothing".
    render(
      <VpnStatusCard
        olts={[olt("10.9.9.9")]}
        peers={[peer({ id: "1", name: "Depok", connected: false })]}
      />,
    );

    expect(screen.queryByText(/OLTs?$/)).not.toBeInTheDocument();
  });

  it("leaves switched-off tunnels out of the ratio but still lists them", () => {
    // Counting a deliberately disabled site as down would report a fault the
    // operator created on purpose.
    render(
      <VpnStatusCard
        olts={[]}
        peers={[
          peer({ id: "1", name: "Depok" }),
          peer({ id: "2", name: "Lama", enabled: false, connected: false }),
        ]}
      />,
    );

    expect(screen.getByText("of 1 sites connected")).toBeInTheDocument();
    expect(screen.getByText("switched off")).toBeInTheDocument();
  });

  it("admits when the status could not be loaded", () => {
    render(<VpnStatusCard peers={undefined} olts={[]} isError />);

    expect(
      screen.getByText("Tunnel status could not be loaded."),
    ).toBeInTheDocument();
  });
});
