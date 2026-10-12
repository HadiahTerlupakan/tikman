import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { RouterInterface } from "@/domain/entities";
import { FlowDestinationsTab } from "./FlowDestinationsTab";

const destinations = vi.fn();
const topIps = vi.fn();
const state = { destinationsError: false };

function iface(id: string, comment: string): RouterInterface {
  return {
    id,
    routerId: "r1",
    name: id,
    comment,
    type: "vlan",
    vlanId: null,
    parentName: null,
    ifIndex: null,
    speedMbps: null,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: null,
    lastOutBps: null,
    lastPolledAt: null,
  };
}

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({
    data: [iface("pop-a", "POP A"), iface("transit", "Transit")],
  }),
  useRouterBgpSessions: () => ({ data: [{ interfaceId: "transit" }] }),
}));

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowDestinations: (...args: unknown[]) => {
    destinations(...args);
    return { data: undefined, isError: state.destinationsError };
  },
  useFlowSummary: () => ({ data: undefined, isLoading: false }),
  useFlowPops: () => ({
    data: [{ popInterfaceId: "pop-a", bytes: 1000, topAsns: [] }],
    isLoading: false,
  }),
  useFlowTopIps: (...args: unknown[]) => {
    topIps(...args);
    return { data: [], isLoading: false };
  },
}));

describe("FlowDestinationsTab", () => {
  beforeEach(() => {
    destinations.mockReset();
    topIps.mockReset();
    state.destinationsError = false;
  });

  it("filters by the POP whose row is clicked", async () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);

    fireEvent.click(screen.getByText("POP A"));

    await waitFor(() =>
      expect(destinations).toHaveBeenLastCalledWith(
        "r1",
        expect.anything(),
        expect.objectContaining({ pop: "pop-a", direction: "download" }),
      ),
    );
  });

  it("turns the top-IP panel off for a range past seven days", async () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);
    expect(screen.queryByText(/hanya disimpan 7 hari/)).toBeNull();

    fireEvent.click(screen.getByText("30 hari"));

    expect(await screen.findByText(/hanya disimpan 7 hari/)).toBeTruthy();
    expect(topIps).toHaveBeenLastCalledWith(
      "r1",
      expect.anything(),
      expect.anything(),
      false,
    );
  });

  it("shows an error, not the empty text, when the destinations request fails", () => {
    state.destinationsError = true;
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);

    expect(screen.getByText("Gagal memuat data tujuan trafik")).toBeTruthy();
    expect(screen.queryByText("Belum ada data di rentang ini")).toBeNull();
  });

  it("explains that NetFlow is off, and stays quiet once it is on", () => {
    const { unmount } = render(
      <FlowDestinationsTab routerId="r1" flowEnabled={false} />,
    );
    expect(screen.getByText(/NetFlow belum diaktifkan/)).toBeTruthy();
    unmount();

    render(<FlowDestinationsTab routerId="r1" flowEnabled />);
    expect(screen.queryByText(/NetFlow belum diaktifkan/)).toBeNull();
  });
});
