import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { RouterInterface } from "@/domain/entities";
import { FlowDestinationsTab } from "./FlowDestinationsTab";

const destinations = vi.fn();
const topIps = vi.fn();
const peering = vi.fn();

function summaryRow(asn: number, name: string, bytes: number) {
  return {
    asn,
    name,
    bytes,
    percent: 1,
    avgBps: 1,
    peakBps: 1,
    upstreams: [],
  };
}

const SUMMARY = {
  totalBytes: 1_000_000,
  peakWindowSeconds: 60,
  rows: Array.from({ length: 12 }, (_, i) =>
    summaryRow(64496 + i, `EXAMPLE-NET-${i}`, 1000 - i),
  ),
};
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
    lastTrafficAt: null,
    silentSince: null,
  };
}

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({
    data: [iface("pop-a", "POP A"), iface("transit", "Transit")],
  }),
  useRouterBgpSessions: () => ({
    data: [{ name: "peer-transit", interfaceId: "transit" }],
  }),
}));

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowDestinations: (...args: unknown[]) => {
    destinations(...args);
    return { data: undefined, isError: state.destinationsError };
  },
  useFlowSummary: () => ({ data: SUMMARY, isLoading: false }),
  useFlowPeering: (...args: unknown[]) => {
    peering(...args);
    return {
      data:
        args[3] === null
          ? undefined
          : {
              asn: 64496,
              name: "EXAMPLE-NET-0",
              totalBytes: 2000,
              stepSeconds: 60,
              peakWindowSeconds: 60,
              rows: [
                {
                  upstreamInterfaceId: "transit",
                  bytes: 2000,
                  percent: 100,
                  avgBps: 26.7,
                  peakBps: 40,
                },
              ],
              series: [],
              pops: [
                {
                  popInterfaceId: "pop-a",
                  bytes: 1500,
                  percent: 75,
                  upstreams: [{ upstreamInterfaceId: "transit", bytes: 1500 }],
                },
                {
                  popInterfaceId: null,
                  bytes: 500,
                  percent: 25,
                  upstreams: [],
                },
              ],
            },
      isLoading: false,
      isError: false,
    };
  },
  useFlowPops: () => ({
    data: [{ popInterfaceId: "pop-a", bytes: 1000, topAsns: [] }],
    isLoading: false,
  }),
  useFlowTopIps: (...args: unknown[]) => {
    topIps(...args);
    return { data: [], isLoading: false };
  },
  useFlowRouteAdvice: () => ({ data: [], isLoading: false }),
  useFlowOwnPrefixes: () => ({ data: [], isLoading: false }),
}));

describe("FlowDestinationsTab", () => {
  beforeEach(() => {
    destinations.mockReset();
    topIps.mockReset();
    peering.mockReset();
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

  it("pages the destination table ten rows at a time", () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);

    expect(screen.getByText("EXAMPLE-NET-9 (AS64505)")).toBeTruthy();
    expect(screen.queryByText("EXAMPLE-NET-10 (AS64506)")).toBeNull();
  });

  it("opens the per-peering split of the destination whose row is clicked", async () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);

    fireEvent.click(screen.getByText("EXAMPLE-NET-0 (AS64496)"));

    expect(
      await screen.findByText("EXAMPLE-NET-0 (AS64496) — per peering"),
    ).toBeTruthy();
    expect(peering).toHaveBeenLastCalledWith(
      "r1",
      expect.anything(),
      expect.objectContaining({ direction: "download" }),
      64496,
    );
    expect(
      screen.getAllByText("peer-transit (Transit)").length,
    ).toBeGreaterThan(0);
  });

  it("lists the POPs that use the destination with their share", async () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);

    fireEvent.click(screen.getByText("EXAMPLE-NET-0 (AS64496)"));

    const drawer = within(
      (await screen.findByText("Per POP (VLAN)")).closest(
        ".ant-drawer-content",
      ) as HTMLElement,
    );
    expect(drawer.getByText("POP A")).toBeTruthy();
    expect(drawer.getByText("75.0%")).toBeTruthy();
    expect(drawer.getByText("Lainnya (di luar blok POP)")).toBeTruthy();
  });

  it("drops the per-POP table while the tab is filtered to one POP", async () => {
    render(<FlowDestinationsTab routerId="r1" flowEnabled />);
    fireEvent.click(screen.getByText("POP A"));

    fireEvent.click(screen.getByText("EXAMPLE-NET-0 (AS64496)"));

    await screen.findByText("EXAMPLE-NET-0 (AS64496) — per peering");
    expect(screen.queryByText("Per POP (VLAN)")).toBeNull();
  });
});
