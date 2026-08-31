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

const peer = {
  id: "peer-1",
  siteId: "site-1",
  name: "Cariu",
  tunnelAddress: "10.88.0.5",
  allowedIps: ["172.30.30.3/32"],
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
    useWireguardPeers.mockReturnValue({ data: [peer] });
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

  it("states the route the OLT needs to reach the trap destination", () => {
    renderPanel();

    expect(textOf("trap-setup-olt")).toContain(
      "ip route 10.88.0.0 255.255.255.0",
    );
  });

  it("addresses the MikroTik peer at the server's real endpoint and key", () => {
    renderPanel();

    const commands = textOf("trap-setup-mikrotik");
    expect(commands).toContain('public-key="SERVERPUB"');
    expect(commands).toContain(
      "endpoint-address=vpn.contoh.id endpoint-port=51820",
    );
    expect(commands).toContain("allowed-address=10.88.0.0/24");
  });

  it("uses the tunnel address the site's peer already holds", () => {
    renderPanel();

    expect(textOf("trap-setup-mikrotik")).toContain(
      "address=10.88.0.5/24 interface=wg-tikman",
    );
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
