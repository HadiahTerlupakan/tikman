import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { MikrotikRouter, RouterInterface } from "@/domain/entities";
import { RouterRack } from "./RouterRack";

const iface = (over: Partial<RouterInterface>): RouterInterface =>
  ({
    id: over.name,
    routerId: "r-1",
    name: "ether1",
    comment: "",
    type: "ether",
    vlanId: null,
    parentName: null,
    speedMbps: 1000,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: 1e6,
    lastOutBps: 1e5,
    silentSince: null,
    ...over,
  }) as RouterInterface;

const baseInterfaces = [
  iface({ name: "ether1", addresses: "192.0.2.1/24" }),
  iface({ name: "ether2", comment: "peer-transit" }),
  iface({ name: "ether3", comment: "pop-a", operStatus: "down" }),
];

const data: {
  interfaces: RouterInterface[] | undefined;
  loading: boolean;
  failed?: boolean;
} = {
  interfaces: baseInterfaces,
  loading: false,
};

const sessionsRequests: unknown[][] = [];

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({
    data: data.interfaces,
    isLoading: data.loading,
    isError: data.failed ?? false,
  }),
  useRouterBgpSessions: (...args: unknown[]) => {
    sessionsRequests.push(args);
    return { data: [] };
  },
}));

const router = (over: Partial<MikrotikRouter>): MikrotikRouter =>
  ({
    id: "r-1",
    name: "border-1",
    host: "192.0.2.1",
    enabled: true,
    status: "up",
    apiStatus: "ok",
    cpuLoad: 12,
    boardName: "RB5009UG+S+IN",
    bgpEstablished: 2,
    bgpTotal: 2,
    silentInterfaces: 0,
    rackPosition: null,
    role: "border",
    ...over,
  }) as MikrotikRouter;

const psu2Failed = (healthAt: string) => ({
  health: [
    { name: "psu1-state", value: 0, unit: 6 },
    { name: "psu2-state", value: 1, unit: 6 },
  ],
  healthAt,
});

function renderRack(routers: MikrotikRouter[]) {
  render(
    <MemoryRouter initialEntries={["/routers"]}>
      <Routes>
        <Route
          path="/routers"
          element={
            <RouterRack routers={routers} isAdmin={false} onEdit={vi.fn()} />
          }
        />
        <Route path="/routers/r-1" element={<div>detail r-1</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("RouterRack", () => {
  beforeEach(() => {
    data.interfaces = baseInterfaces;
    data.loading = false;
    data.failed = false;
    sessionsRequests.length = 0;
  });

  it("tapes a BRAS unit, counts its subscribers and leaves BGP alone", () => {
    renderRack([
      router({
        name: "bras-1",
        role: "bras",
        bgpEstablished: 0,
        bgpTotal: 2,
        bras: {
          sessions: 1842,
          cores: [],
          pools: [],
          radius: [],
          topUsers: [],
          queuesFound: true,
        },
      }),
    ]);
    expect(document.querySelector(".fp-roletape-bras")?.textContent).toBe(
      "BRAS",
    );
    expect(screen.getByText(/1\.842 online/)).toBeTruthy();
    expect(sessionsRequests.every(([, opts]) => opts === false)).toBe(true);
    expect(screen.queryByText(/sesi BGP mati/)).toBeNull();
  });

  it("tapes a border unit as BORDER", () => {
    renderRack([router({})]);
    expect(document.querySelector(".fp-roletape-border")?.textContent).toBe(
      "BORDER",
    );
  });

  it("leaves a blank panel where no router sits between two positions", () => {
    renderRack([
      router({ id: "r-1", name: "border-1", rackPosition: 1 }),
      router({ id: "r-2", name: "border-2", rackPosition: 3 }),
    ]);
    expect(screen.getAllByTestId("rack-blank")).toHaveLength(1);
  });

  it("shows an unreachable router with every LED off", () => {
    renderRack([
      router({ status: "down", ...psu2Failed(new Date().toISOString()) }),
    ]);
    const unit = screen.getByRole("link", {
      name: "Buka border-1, Tidak terjangkau",
    });
    expect(
      unit.querySelector(".fp-led-up, .fp-led-warn, .fp-led-down"),
    ).toBeNull();
  });

  it("names no live state or rate on a port of an unreachable router", () => {
    renderRack([router({ status: "down" })]);
    const port = document.querySelector('[data-port="ether2"]');
    expect(port?.getAttribute("title")).toBe(
      "ether2 · peer-transit · Tidak terjangkau",
    );
  });

  it("names the ports in trouble on a reachable router", () => {
    renderRack([router({ bgpEstablished: 1, bgpTotal: 2 })]);
    expect(
      screen.getByRole("link", {
        name: "Buka border-1, 1 port mati, 1 sesi BGP mati",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("RB5009UG+S+IN · CPU 12% · BGP 1/2"),
    ).toBeInTheDocument();
  });

  it("names a failed PSU among the troubles", () => {
    renderRack([router(psu2Failed(new Date().toISOString()))]);
    expect(
      screen.getByRole("link", {
        name: "Buka border-1, 1 port mati, PSU2 fail",
      }),
    ).toBeInTheDocument();
  });

  it("names no PSU failure from a reading that is no longer fresh", () => {
    renderRack([router(psu2Failed("2026-01-01T00:00:00Z"))]);
    expect(
      screen.getByRole("link", { name: "Buka border-1, 1 port mati" }),
    ).toBeInTheDocument();
  });

  it("darkens the PWR LEDs when the reading goes stale while the rack is open", () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    try {
      vi.setSystemTime(new Date("2026-11-19T12:00:00Z"));
      renderRack([router(psu2Failed("2026-11-19T12:00:00Z"))]);
      const pwr1 = () => screen.getByText("PWR1").querySelector("i");
      expect(pwr1()?.className).toBe("fp-led-up");

      act(() => vi.advanceTimersByTime(6 * 60 * 1000));

      expect(pwr1()?.className).toBe("");
      expect(
        screen.getByRole("link", { name: "Buka border-1, 1 port mati" }),
      ).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("counts no interface that is not drawn as a port", () => {
    data.interfaces = [
      ...baseInterfaces.slice(0, 2),
      iface({
        name: "wg0",
        type: "wg",
        comment: "tunnel-a",
        operStatus: "down",
      }),
      iface({ name: "bridge1", type: "bridge", silentSince: "2026-11-19" }),
    ];
    renderRack([router({})]);
    expect(screen.getByText("Semua normal")).toBeInTheDocument();
  });

  it("does not call a router without interface data healthy", () => {
    data.interfaces = [iface({ name: "wg0", type: "wg", comment: "tunnel-a" })];
    renderRack([router({})]);
    expect(screen.getByText("Belum ada data interface")).toBeInTheDocument();
    expect(screen.queryByText("Semua normal")).toBeNull();
  });

  it("claims nothing about the interfaces while they load", () => {
    data.interfaces = undefined;
    data.loading = true;
    renderRack([router({})]);
    expect(screen.queryByText("Belum ada data interface")).toBeNull();
    expect(screen.queryByText("Semua normal")).toBeNull();
  });

  it("says the interfaces failed to load rather than that none exist", () => {
    data.interfaces = undefined;
    data.failed = true;
    renderRack([router({})]);
    expect(screen.getByText("Data interface gagal dimuat")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada data interface")).toBeNull();
  });

  it("gives the ports inside a unit no tab stop of their own", () => {
    renderRack([router({})]);
    const unit = screen.getByRole("link", { name: /^Buka border-1/ });
    expect(within(unit).queryAllByRole("button")).toEqual([]);
    expect(unit.querySelector("button, [tabindex]")).toBeNull();
  });

  describe("at a known width", () => {
    let spy: ReturnType<typeof vi.spyOn>;
    beforeEach(() => {
      spy = vi
        .spyOn(HTMLElement.prototype, "clientWidth", "get")
        .mockReturnValue(600);
    });
    afterEach(() => spy.mockRestore());

    it("makes a blank panel as tall as a unit", () => {
      renderRack([
        router({ id: "r-1", name: "border-1", rackPosition: 1 }),
        router({ id: "r-2", name: "border-2", rackPosition: 3 }),
      ]);
      const unit = screen.getByRole("link", { name: /^Buka border-1/ });
      const blank = screen.getByTestId("rack-blank");
      expect(unit.style.height).not.toBe("");
      expect(blank.style.height).toBe(unit.style.height);
    });
  });

  it("reads the BGP count as stale while the router's API fails", () => {
    renderRack([router({ apiStatus: "error" })]);
    expect(
      screen.getByText("RB5009UG+S+IN · CPU 12% · BGP usang"),
    ).toBeInTheDocument();
  });

  it("opens the router detail when a unit is clicked", async () => {
    renderRack([router({})]);
    const unit = screen.getByRole("link", { name: /^Buka border-1/ });
    await userEvent.click(within(unit).getByText("border-1"));
    expect(screen.getByText("detail r-1")).toBeInTheDocument();
  });
});
