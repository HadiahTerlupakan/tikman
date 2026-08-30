import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { OltStatus, UserRole } from "@/domain/entities";
import type { DashboardStats } from "@/domain/entities";
import DashboardPage from "../Dashboard";

interface QueryLike<T> {
  data: T | undefined;
  isLoading: boolean;
  error: unknown;
}

const noOnts = {
  total: 0,
  online: 0,
  offline: 0,
  los: 0,
  dyingGasp: 0,
  unknown: 0,
};

const state: {
  sites: QueryLike<{ id: string }[]>;
  olts: QueryLike<
    { id: string; name: string; status: OltStatus; ipAddress?: string }[]
  >;
  stats: QueryLike<DashboardStats>;
  peers: QueryLike<
    {
      id: string;
      name: string;
      enabled: boolean;
      connected: boolean;
      allowedIps: string[];
      lastHandshakeAt: string | null;
    }[]
  >;
} = {
  sites: { data: [{ id: "s1" }], isLoading: false, error: null },
  olts: { data: [], isLoading: false, error: null },
  stats: {
    data: { onts: noOnts, olts: [], weakestSignals: [] },
    isLoading: false,
    error: null,
  },
  peers: { data: [], isLoading: false, error: null },
};

vi.mock("@/application/hooks", () => ({
  useUsers: () => ({ data: [{ id: "u1" }], isLoading: false }),
  useSites: () => state.sites,
  useOlts: () => state.olts,
  useDashboardStats: () => ({
    ...state.stats,
    isFetching: false,
    dataUpdatedAt: 0,
  }),
  useWireguardPeers: () => state.peers,
  useHealth: () => ({
    data: {
      status: "healthy",
      dependencies: { database: "up", redis: "up", worker: "up" },
    },
    isLoading: false,
  }),
}));

let role: UserRole = UserRole.ADMIN;

vi.mock("@/application/stores", () => ({
  useAuthStore: (selector: (s: unknown) => unknown) =>
    selector({ user: { role } }),
}));

describe("DashboardPage", () => {
  beforeEach(() => {
    role = UserRole.ADMIN;
    state.sites = { data: [{ id: "s1" }], isLoading: false, error: null };
    state.olts = {
      data: [
        { id: "o1", name: "Depok", status: OltStatus.ONLINE },
        { id: "o2", name: "Bekasi", status: OltStatus.ERROR },
      ],
      isLoading: false,
      error: null,
    };
    state.stats = {
      data: {
        onts: { ...noOnts, total: 2, online: 1, los: 1 },
        olts: [
          {
            oltId: "o1",
            oltName: "Depok",
            oltStatus: OltStatus.ONLINE,
            ontTotal: 1,
            online: 1,
            impaired: 0,
          },
          {
            oltId: "o2",
            oltName: "Bekasi",
            oltStatus: OltStatus.ERROR,
            ontTotal: 1,
            online: 0,
            impaired: 1,
          },
        ],
        weakestSignals: [],
      },
      isLoading: false,
      error: null,
    };
    state.peers = { data: [], isLoading: false, error: null };
  });

  it("summarises OLT and ONT counts from the loaded data", () => {
    render(<DashboardPage />);

    expect(screen.getByText("Total OLTs")).toBeInTheDocument();
    expect(screen.getByText("Total ONTs")).toBeInTheDocument();
    expect(screen.getByText("50% online")).toBeInTheDocument();
    expect(screen.getByText("1 of 2 online")).toBeInTheDocument();
  });

  it("breaks the counts down to the OLT the operator has to visit", () => {
    render(<DashboardPage />);

    expect(screen.getByText("Depok")).toBeInTheDocument();
    expect(screen.getByText("Bekasi")).toBeInTheDocument();
    // Bekasi carries the only impaired ONT, so it sorts above the healthy OLT.
    const names = screen
      .getAllByText(/^(Depok|Bekasi)$/)
      .map((node) => node.textContent);
    expect(names[0]).toBe("Bekasi");
  });

  it("reports the whole network rather than a page of it", () => {
    // The page used to count the rows one request returned, so a 930-ONT
    // network read as 500. The figures come from the server's own count now,
    // and no ONT row reaches the browser at all.
    state.stats = {
      data: {
        onts: { ...noOnts, total: 930, online: 462, offline: 446, los: 22 },
        olts: [],
        weakestSignals: [],
      },
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("930")).toBeInTheDocument();
    expect(screen.getByText("462 of 930 online")).toBeInTheDocument();
  });

  it("lists the online ONT receiving the least light", () => {
    state.stats = {
      data: {
        onts: { ...noOnts, total: 1, online: 1 },
        olts: [],
        weakestSignals: [
          {
            id: "t1",
            name: "Pelanggan A",
            serialNumber: "ZTEG1",
            oltName: "Depok",
            rxPower: -28.4,
          },
        ],
      },
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("Pelanggan A")).toBeInTheDocument();
    expect(screen.getByText("-28.4 dBm")).toBeInTheDocument();
  });

  it("reports which site tunnels are down and what they cut off", () => {
    state.peers = {
      data: [
        {
          id: "p1",
          name: "Depok tunnel",
          enabled: true,
          connected: true,
          allowedIps: ["10.9.9.0/24"],
          lastHandshakeAt: new Date().toISOString(),
        },
        {
          id: "p2",
          name: "Bekasi tunnel",
          enabled: true,
          connected: false,
          allowedIps: ["192.168.220.0/24"],
          lastHandshakeAt: null,
        },
      ],
      isLoading: false,
      error: null,
    };
    state.olts = {
      data: [
        {
          id: "o1",
          name: "Depok",
          status: OltStatus.ONLINE,
          ipAddress: "192.168.220.22",
        },
      ],
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("of 2 sites connected")).toBeInTheDocument();
    expect(screen.getByText("Bekasi tunnel")).toBeInTheDocument();
    expect(screen.getByText("never connected")).toBeInTheDocument();
    // The OLT the rest of the page shows as unreachable sits behind it.
    expect(screen.getByText("1 OLT")).toBeInTheDocument();
  });

  it("warns which resource failed and renders a dash rather than zero", () => {
    // Rendering 0 for a failed query would tell the operator there are no OLTs
    // in error, which is the opposite of "we do not know".
    state.olts = {
      data: undefined,
      isLoading: false,
      error: new Error("boom"),
    };

    render(<DashboardPage />);

    expect(screen.getByText(/Could not load OLTs/)).toBeInTheDocument();
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  it("invites the operator to register hardware when nothing exists yet", () => {
    state.olts = { data: [], isLoading: false, error: null };
    state.stats = {
      data: { onts: noOnts, olts: [], weakestSignals: [] },
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("No OLTs registered yet")).toBeInTheDocument();
    expect(screen.getByText("No ONTs registered yet")).toBeInTheDocument();
    expect(screen.getByText("No ONTs to measure")).toBeInTheDocument();
  });

  it("shows the user count to admins", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Total Users")).toBeInTheDocument();
  });

  it("hides the user count from non-admins", () => {
    role = UserRole.TECHNICIAN;

    render(<DashboardPage />);

    expect(screen.queryByText("Total Users")).not.toBeInTheDocument();
    expect(screen.getByText("Total OLTs")).toBeInTheDocument();
  });
});
