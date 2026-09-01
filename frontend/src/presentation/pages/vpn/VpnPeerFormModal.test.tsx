import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VpnPeerFormModal } from "./VpnPeerFormModal";

const suggestionFor = vi.hoisted(() => vi.fn());
const createPeer = vi.hoisted(() => vi.fn());
const updatePeer = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useSites: () => ({
    data: [
      { id: "site-1", name: "Site Bandung" },
      { id: "site-2", name: "Site Bogor" },
    ],
  }),
  useSuggestedSubnets: (siteId?: string) => ({ data: suggestionFor(siteId) }),
  useCreateWireguardPeer: () => ({
    mutateAsync: createPeer,
    isPending: false,
    isError: false,
    error: null,
  }),
  useUpdateWireguardPeer: () => ({
    mutateAsync: updatePeer,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

async function chooseSite(name: string) {
  fireEvent.mouseDown(screen.getByLabelText("Site"));
  fireEvent.click(await screen.findByTitle(name));
}

function subnetField() {
  return screen.getByLabelText("Subnet lokal di site");
}

function nameField() {
  return screen.getByLabelText("Nama tunnel");
}

describe("VpnPeerFormModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createPeer.mockResolvedValue({});
    updatePeer.mockResolvedValue({});
    // Site Bogor has no registered OLTs, so the API has nothing to suggest.
    suggestionFor.mockImplementation((siteId?: string) =>
      siteId === "site-1" ? ["10.10.10.0/24"] : undefined,
    );
  });

  // A suggestion left in the field would be submitted as another site's local
  // subnet, routing that site's traffic at the wrong peer.
  it("clears the previous site's suggestion when the new site has none", async () => {
    render(<VpnPeerFormModal open onClose={vi.fn()} />);

    await chooseSite("Site Bandung");
    await waitFor(() => expect(subnetField()).toHaveValue("10.10.10.0/24"));

    await chooseSite("Site Bogor");
    await waitFor(() => expect(subnetField()).toHaveValue(""));
  });

  it("ignores a trailing comma instead of submitting an empty subnet", async () => {
    render(<VpnPeerFormModal open onClose={vi.fn()} />);

    await chooseSite("Site Bandung");
    await waitFor(() => expect(subnetField()).toHaveValue("10.10.10.0/24"));
    await userEvent.type(subnetField(), ", ");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(createPeer).toHaveBeenCalled());
    expect(createPeer.mock.calls[0][0]).toMatchObject({
      siteId: "site-1",
      name: "Site Bandung",
      allowedIps: ["10.10.10.0/24"],
    });
  });

  // A site with two POPs registers two tunnels, and both would arrive named
  // after the site — indistinguishable in the table an operator reads to find
  // which router is down.
  it("submits the tunnel name the operator typed, not the site's", async () => {
    render(<VpnPeerFormModal open onClose={vi.fn()} />);

    await chooseSite("Site Bandung");
    await waitFor(() => expect(nameField()).toHaveValue("Site Bandung"));

    await userEvent.clear(nameField());
    await userEvent.type(nameField(), "Site Bandung POP 2");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(createPeer).toHaveBeenCalled());
    expect(createPeer.mock.calls[0][0]).toMatchObject({
      siteId: "site-1",
      name: "Site Bandung POP 2",
    });
  });

  it("prefills an edited peer and updates it instead of creating another", async () => {
    render(
      <VpnPeerFormModal
        open
        peer={{
          id: "peer-1",
          siteId: "site-1",
          name: "Site Bandung",
          tunnelAddress: "10.88.0.2",
          allowedIps: ["10.10.10.0/24", "192.168.88.0/24"],
          persistentKeepalive: 25,
          enabled: true,
          connected: true,
          lastHandshakeAt: null,
          rxBytes: 0,
          txBytes: 0,
          createdAt: "",
          updatedAt: "",
        }}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() =>
      expect(subnetField()).toHaveValue("10.10.10.0/24, 192.168.88.0/24"),
    );

    await userEvent.clear(subnetField());
    await userEvent.type(subnetField(), "10.10.11.0/24");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(updatePeer).toHaveBeenCalled());
    expect(createPeer).not.toHaveBeenCalled();
    expect(updatePeer.mock.calls[0][0]).toEqual({
      id: "peer-1",
      data: { name: "Site Bandung", allowedIps: ["10.10.11.0/24"] },
    });
  });
});
