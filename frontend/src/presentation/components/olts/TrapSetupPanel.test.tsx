import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { WireguardPeer, WireguardServer } from "@/domain/entities";
import { TrapSetupPanel } from "./TrapSetupPanel";

const useWireguardServer = vi.hoisted(() =>
  vi.fn<() => { data: WireguardServer | undefined }>(),
);
const useWireguardPeers = vi.hoisted(() =>
  vi.fn<() => { data: WireguardPeer[] | undefined }>(),
);

vi.mock("@/application/hooks", () => ({
  useWireguardServer,
  useWireguardPeers,
}));

const server: WireguardServer = {
  id: "srv-1",
  interfaceName: "wg0",
  listenPort: 51820,
  publicKey: "SERVERPUB",
  endpointHost: "vpn.contoh.id",
  tunnelSubnet: "10.88.0.0/24",
  address: "10.88.0.1",
};

/** A site whose peer exists but has never handshaken: the tunnel is still to build. */
const newPeer = {
  id: "peer-1",
  siteId: "site-1",
  name: "Site Baru",
  tunnelAddress: "10.88.0.5",
  allowedIps: ["172.30.30.3/32"],
  connected: false,
  lastHandshakeAt: null,
} as WireguardPeer;

const connectedPeer = {
  ...newPeer,
  connected: true,
  lastHandshakeAt: new Date().toISOString(),
} as WireguardPeer;

function renderPanel(
  props: Partial<Parameters<typeof TrapSetupPanel>[0]> = {},
) {
  return render(
    <TrapSetupPanel
      siteId="site-1"
      ipAddress="172.30.30.3"
      snmpCommunity="public"
      {...props}
    />,
  );
}

const textOf = (testId: string) => screen.getByTestId(testId).textContent ?? "";

describe("TrapSetupPanel", () => {
  beforeEach(() => {
    useWireguardServer.mockReturnValue({ data: server });
    useWireguardPeers.mockReturnValue({ data: [newPeer] });
  });

  it("builds the OLT trap command from the server and the form", () => {
    renderPanel();

    expect(textOf("trap-setup-olt")).toContain(
      "snmp-server host 10.88.0.1 version 2c public enable NOTIFICATIONS " +
        "target-addr-name EMS_10.88.0.1 isnmsserver udp-port 162 " +
        "trap-report-compatibility v20",
    );
  });

  it("falls back to the default community when the field is empty", () => {
    renderPanel({ snmpCommunity: "" });

    // The form defaults SNMP community to public on submit, so an empty field
    // must not render a command carrying a blank community that gets pasted.
    expect(textOf("trap-setup-olt")).toContain("version 2c public enable");
  });

  it("routes the OLT to the trap destination via its subnet's first host", () => {
    renderPanel();

    expect(textOf("trap-setup-olt")).toContain(
      "ip route 10.88.0.0 255.255.255.0 172.30.30.1",
    );
  });

  it("keeps a placeholder gateway while no OLT address is entered", () => {
    renderPanel({ ipAddress: "" });

    // With no address there is nothing to derive from, and printing some other
    // site's gateway would be worse than asking for it.
    expect(textOf("trap-setup-olt")).toContain("<gateway-LAN>");
  });

  it("asks for nothing on the MikroTik once the site's tunnel is up", () => {
    useWireguardPeers.mockReturnValue({ data: [connectedPeer] });
    renderPanel();

    // Traps ride the tunnel the poller already reaches this OLT through. Adding
    // a second wireguard interface is not a no-op — it is a second tunnel.
    expect(screen.queryByTestId("trap-setup-mikrotik")).not.toBeInTheDocument();
    expect(screen.getByText(/sudah aktif/i)).toBeInTheDocument();
  });

  it("shows the MikroTik setup only for a site whose tunnel is not up yet", () => {
    renderPanel();

    const commands = textOf("trap-setup-mikrotik");
    expect(commands).toContain('public-key="SERVERPUB"');
    expect(commands).toContain(
      "endpoint-address=vpn.contoh.id endpoint-port=51820",
    );
    expect(commands).toContain("allowed-address=10.88.0.0/24");
    expect(commands).toContain("address=10.88.0.5/24 interface=wg-tikman");
  });

  it("says the site has no peer yet rather than inventing a tunnel address", () => {
    useWireguardPeers.mockReturnValue({ data: [] });
    renderPanel();

    // Every site needs its own address inside the tunnel subnet. Printing a
    // guessed one would collide with whichever site already holds it.
    expect(screen.getByText(/belum punya peer VPN/i)).toBeInTheDocument();
    expect(screen.queryByTestId("trap-setup-mikrotik")).not.toBeInTheDocument();
  });

  it("renders nothing until the VPN server is known", () => {
    useWireguardServer.mockReturnValue({ data: undefined });
    const { container } = renderPanel();

    expect(container).toBeEmptyDOMElement();
  });
});
