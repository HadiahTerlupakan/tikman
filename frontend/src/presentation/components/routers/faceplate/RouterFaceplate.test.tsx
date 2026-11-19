import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import { RouterFaceplate } from "./RouterFaceplate";

const iface = (over: Partial<RouterInterface>): RouterInterface =>
  ({
    id: over.name,
    routerId: "r1",
    name: "ether1",
    comment: "",
    type: "ether",
    vlanId: null,
    parentName: null,
    speedMbps: 10000,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: 1e6,
    lastOutBps: 1e5,
    silentSince: null,
    ...over,
  }) as RouterInterface;

const vlan = (name: string, vlanId: number, comment: string) =>
  iface({
    name,
    vlanId,
    comment,
    type: "vlan",
    parentName: "sfp-sfpplus2",
  });

const session = (over: Partial<RouterBgpSession>): RouterBgpSession =>
  ({
    id: over.name,
    name: "peer",
    remoteAddress: "192.0.2.9",
    remoteAs: 64500,
    state: "established",
    prefixCount: 10,
    interfaceId: null,
    ...over,
  }) as RouterBgpSession;

const baseInterfaces = [
  iface({ name: "ether1", addresses: "192.0.2.1/24" }),
  iface({ name: "sfp-sfpplus1", comment: "peer-transit", lastInBps: 3e9 }),
  iface({ name: "sfp-sfpplus2", comment: "trunk-metro" }),
  vlan("vlan201", 201, "peer-ix"),
  vlan("vlan101", 101, "pop-a"),
  vlan("vlan102", 102, "pop-b"),
  vlan("vlan103", 103, "pop-c"),
  vlan("vlan104", 104, "pop-d"),
];

const sessions = [
  session({ name: "peer-transit-v4", interfaceId: "sfp-sfpplus1" }),
  session({ name: "peer-ix-rs1", interfaceId: "vlan201", state: "down" }),
];

const data: {
  interfaces: RouterInterface[] | undefined;
  sessions: RouterBgpSession[];
  loading: boolean;
} = { interfaces: baseInterfaces, sessions, loading: false };

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({
    data: data.interfaces,
    isLoading: data.loading,
  }),
  useRouterBgpSessions: () => ({ data: data.sessions }),
  useInterfaceTraffic: () => ({ data: [] }),
}));

const router = {
  id: "r1",
  name: "border-1",
  host: "192.0.2.1",
  enabled: true,
  status: "up",
  apiStatus: "ok",
  boardName: "CCR2116-12G-4S+",
} as MikrotikRouter;

const label = (name: RegExp) => screen.getByRole("button", { name });

describe("RouterFaceplate", () => {
  beforeEach(() => {
    data.interfaces = baseInterfaces;
    data.sessions = sessions;
    data.loading = false;
  });

  it("lists a trunk's first three VLANs and counts the rest", () => {
    render(<RouterFaceplate router={router} />);

    const trunk = label(/^trunk-metro/);
    const rows = trunk.querySelectorAll(".fp-vl:not(.fp-vl-more)");
    expect(rows).toHaveLength(3);
    expect(within(trunk).getByText("+2 VLAN lain")).toBeTruthy();
    expect(rows[0].querySelector(".fp-vl-dot")).not.toBeNull();
    expect(rows[1].querySelector(".fp-vl-dot")).toBeNull();
  });

  it("opens the trunk first and shows a port's sessions when its label is clicked", () => {
    render(<RouterFaceplate router={router} />);

    expect(screen.getByRole("heading", { name: "trunk-metro" })).toBeTruthy();
    expect(screen.queryByText("peer-transit-v4")).toBeNull();

    fireEvent.click(label(/^peer-transit/));

    expect(screen.getByRole("heading", { name: "peer-transit" })).toBeTruthy();
    expect(screen.getByText("peer-transit-v4")).toBeTruthy();
    expect(screen.getByText("Tidak ada VLAN di port ini.")).toBeTruthy();
  });

  it("keeps the chosen port selected when the data is refetched", () => {
    const { rerender } = render(<RouterFaceplate router={router} />);
    fireEvent.click(label(/^peer-transit/));

    data.interfaces = baseInterfaces.map((i) => ({ ...i, lastInBps: 5e6 }));
    data.sessions = sessions.map((s) => ({ ...s }));
    rerender(<RouterFaceplate router={router} />);

    expect(screen.getByRole("heading", { name: "peer-transit" })).toBeTruthy();
    expect(label(/^peer-transit/).className).toContain("fp-tag-selected");
  });

  it("warns that sessions are last-known while the router API fails", () => {
    render(<RouterFaceplate router={{ ...router, apiStatus: "error" }} />);

    expect(
      screen.getByText("Data BGP usang: API router sedang gagal"),
    ).toBeTruthy();
  });

  it("says so when the router has no interfaces yet", () => {
    data.interfaces = [];
    data.sessions = [];
    render(<RouterFaceplate router={router} />);

    expect(screen.getByText("Belum ada data interface")).toBeTruthy();
  });

  it("claims no missing data while the interfaces load", () => {
    data.interfaces = undefined;
    data.loading = true;
    render(<RouterFaceplate router={router} />);

    expect(screen.queryByText("Belum ada data interface")).toBeNull();
  });

  it("presents an unreachable router's sessions and VLANs as last read", () => {
    render(<RouterFaceplate router={{ ...router, status: "down" }} />);

    expect(
      screen.getByText(
        "Router tidak terjangkau: data di bawah adalah pembacaan terakhir",
      ),
    ).toBeTruthy();
    fireEvent.click(label(/^peer-transit/));
    expect(document.querySelector(".ant-tag-green")).toBeNull();
    fireEvent.click(label(/^trunk-metro/));
    const vlanRow = screen
      .getAllByRole("row")
      .find((row) => row.textContent?.includes("pop-a"));
    expect(vlanRow?.lastElementChild?.textContent).toBe("—");
  });

  it("lists a VLAN that has no role on its port", () => {
    data.interfaces = [
      ...baseInterfaces.slice(0, 2),
      iface({ name: "sfp-sfpplus2", comment: "peer-ix" }),
      { ...vlan("vlan300", 300, ""), operStatus: "down" },
    ];
    render(<RouterFaceplate router={router} />);

    fireEvent.click(label(/^peer-ix/));
    expect(screen.queryByText("Tidak ada VLAN di port ini.")).toBeNull();
    const rows = screen.getAllByRole("row").map((row) => row.textContent);
    expect(rows.some((text) => text?.startsWith("300"))).toBe(true);
  });

  it("says when the port layout was made from interface names", () => {
    render(
      <RouterFaceplate router={{ ...router, boardName: "RB5009UG+S+IN" }} />,
    );

    expect(
      screen.getByText(
        "Model RB5009UG+S+IN belum punya cetakan, jadi susunan port dibuat otomatis dari nama interface.",
      ),
    ).toBeTruthy();
  });

  it("adds no layout note to a board with a template", () => {
    render(<RouterFaceplate router={router} />);

    expect(screen.queryByText(/belum punya cetakan/)).toBeNull();
  });
});
