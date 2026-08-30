import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { OltStatus, OntStatus, UserRole } from "@/domain/entities";
import DashboardPage from "../Dashboard";

interface QueryLike<T> {
  data: T | undefined;
  isLoading: boolean;
  error: unknown;
}

interface OntRow {
  id: string;
  status: OntStatus;
  oltId?: string;
  oltName?: string;
  name?: string;
  serialNumber?: string;
  rxPower?: number | null;
}

const state: {
  sites: QueryLike<{ id: string }[]>;
  olts: QueryLike<{ id: string; name: string; status: OltStatus }[]>;
  onts: QueryLike<{ data: OntRow[]; total: number }>;
  peers: QueryLike<
    { id: string; name: string; enabled: boolean; connected: boolean }[]
  >;
} = {
  sites: { data: [{ id: "s1" }], isLoading: false, error: null },
  olts: { data: [], isLoading: false, error: null },
  onts: { data: { data: [], total: 0 }, isLoading: false, error: null },
  peers: { data: [], isLoading: false, error: null },
};

vi.mock("@/application/hooks", () => ({
  useUsers: () => ({ data: [{ id: "u1" }], isLoading: false }),
  useSites: () => state.sites,
  useOlts: () => state.olts,
  useOnts: () => ({ ...state.onts, isFetching: false, dataUpdatedAt: 0 }),
  useWireguardPeers: () => state.peers,
  useHealth: () => ({
    data: {
      status: "healthy",
      dependencies: { database: "up", redis: "up" },
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
    state.onts = {
      data: {
        data: [
          { id: "t1", status: OntStatus.ONLINE, oltId: "o1" },
          { id: "t2", status: OntStatus.LOS, oltId: "o2" },
        ],
        total: 2,
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

  it("says so when the breakdown covers only part of the network", () => {
    // Silently describing 500 of 900 ONTs would understate every fault count.
    state.onts = {
      data: {
        data: [{ id: "t1", status: OntStatus.ONLINE, oltId: "o1" }],
        total: 900,
      },
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(
      screen.getByText(/covers the 1 most recent ONTs of 900/),
    ).toBeInTheDocument();
  });

  it("lists the online ONT receiving the least light", () => {
    state.onts = {
      data: {
        data: [
          {
            id: "t1",
            status: OntStatus.ONLINE,
            oltId: "o1",
            oltName: "Depok",
            name: "Pelanggan A",
            serialNumber: "ZTEG1",
            rxPower: -28.4,
          },
        ],
        total: 1,
      },
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("Pelanggan A")).toBeInTheDocument();
    expect(screen.getByText("-28.4 dBm")).toBeInTheDocument();
  });

  it("reports which site tunnels are down", () => {
    state.peers = {
      data: [
        { id: "p1", name: "Depok", enabled: true, connected: true },
        { id: "p2", name: "Bekasi", enabled: true, connected: false },
      ],
      isLoading: false,
      error: null,
    };

    render(<DashboardPage />);

    expect(screen.getByText("of 2 sites connected")).toBeInTheDocument();
    expect(screen.getByText(/Down: Bekasi/)).toBeInTheDocument();
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
    state.onts = {
      data: { data: [], total: 0 },
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
