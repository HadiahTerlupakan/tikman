import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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

const interfaces = [
  iface({ name: "ether1", addresses: "192.0.2.1/24" }),
  iface({ name: "ether2", comment: "peer-transit" }),
  iface({ name: "ether3", comment: "pop-a", operStatus: "down" }),
];

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({ data: interfaces }),
  useRouterBgpSessions: () => ({ data: [] }),
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
    ...over,
  }) as MikrotikRouter;

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
  it("leaves a blank panel where no router sits between two positions", () => {
    renderRack([
      router({ id: "r-1", name: "border-1", rackPosition: 1 }),
      router({ id: "r-2", name: "border-2", rackPosition: 3 }),
    ]);
    expect(screen.getAllByTestId("rack-blank")).toHaveLength(1);
  });

  it("shows an unreachable router with every LED off", () => {
    renderRack([router({ status: "down" })]);
    const unit = screen.getByRole("link", { name: "Buka border-1" });
    expect(screen.getByText("Tidak terjangkau")).toBeInTheDocument();
    expect(
      unit.querySelector(".fp-led-up, .fp-led-warn, .fp-led-down"),
    ).toBeNull();
  });

  it("names the ports in trouble on a reachable router", () => {
    renderRack([router({ bgpEstablished: 1, bgpTotal: 2 })]);
    expect(
      screen.getByText("1 port mati, 1 sesi BGP mati"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("RB5009UG+S+IN · CPU 12% · BGP 1/2"),
    ).toBeInTheDocument();
  });

  it("reads the BGP count as stale while the router's API fails", () => {
    renderRack([router({ apiStatus: "error" })]);
    expect(
      screen.getByText("RB5009UG+S+IN · CPU 12% · BGP usang"),
    ).toBeInTheDocument();
  });

  it("opens the router detail when a unit is clicked", async () => {
    renderRack([router({})]);
    const unit = screen.getByRole("link", { name: "Buka border-1" });
    await userEvent.click(within(unit).getByText("border-1"));
    expect(screen.getByText("detail r-1")).toBeInTheDocument();
  });
});
