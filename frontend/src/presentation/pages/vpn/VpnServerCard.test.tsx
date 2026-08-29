import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/infrastructure/http";
import type { WireguardServer } from "@/domain/entities";
import { VpnServerCard } from "./VpnServerCard";

const useWireguardServer = vi.hoisted(() =>
  vi.fn<
    () => {
      data: WireguardServer | undefined;
      isLoading: boolean;
      error: unknown;
    }
  >(),
);

const saveServer = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useWireguardServer,
  useSaveWireguardServer: () => ({ mutate: saveServer, isPending: false }),
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

describe("VpnServerCard", () => {
  it("offers the one-time setup form when the server has never been configured", () => {
    useWireguardServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new ApiError(404, "NOT_FOUND"),
    });

    render(<VpnServerCard />);

    expect(
      screen.getByRole("button", { name: "Aktifkan" }),
    ).toBeInTheDocument();
  });

  // Submitting the setup form again would overwrite the endpoint and port of a
  // server that is already carrying every site's tunnel.
  it("does not offer the setup form when the server merely failed to load", () => {
    useWireguardServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new ApiError(500, "LOAD_FAILED"),
    });

    render(<VpnServerCard />);

    expect(
      screen.queryByRole("button", { name: "Aktifkan" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Gagal memuat pengaturan server VPN"),
    ).toBeInTheDocument();
  });

  it("lets the operator edit an existing server without deleting it", async () => {
    useWireguardServer.mockReturnValue({
      data: server,
      isLoading: false,
      error: null,
    });

    render(<VpnServerCard />);
    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(screen.getByLabelText("Alamat publik VPS")).toHaveValue(
      "vpn.contoh.id",
    );
    expect(screen.getByRole("button", { name: "Simpan" })).toBeInTheDocument();
  });
});
