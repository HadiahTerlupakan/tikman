import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { OltStatus, OntStatus, UserRole } from "@/domain/entities";
import DashboardPage from "../Dashboard";

interface QueryLike<T> {
  data: T | undefined;
  isLoading: boolean;
  error: unknown;
}

const state: {
  sites: QueryLike<{ id: string }[]>;
  olts: QueryLike<{ id: string; status: OltStatus }[]>;
  onts: QueryLike<{ data: { id: string; status: OntStatus }[]; total: number }>;
} = {
  sites: { data: [{ id: "s1" }], isLoading: false, error: null },
  olts: {
    data: [
      { id: "o1", status: OltStatus.ONLINE },
      { id: "o2", status: OltStatus.ERROR },
    ],
    isLoading: false,
    error: null,
  },
  onts: {
    data: {
      data: [
        { id: "t1", status: OntStatus.ONLINE },
        { id: "t2", status: OntStatus.LOS },
      ],
      total: 2,
    },
    isLoading: false,
    error: null,
  },
};

vi.mock("@/application/hooks", () => ({
  useUsers: () => ({ data: [{ id: "u1" }], isLoading: false }),
  useSites: () => state.sites,
  useOlts: () => state.olts,
  useOnts: () => state.onts,
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
        { id: "o1", status: OltStatus.ONLINE },
        { id: "o2", status: OltStatus.ERROR },
      ],
      isLoading: false,
      error: null,
    };
    state.onts = {
      data: {
        data: [
          { id: "t1", status: OntStatus.ONLINE },
          { id: "t2", status: OntStatus.LOS },
        ],
        total: 2,
      },
      isLoading: false,
      error: null,
    };
  });

  it("summarises OLT and ONT counts from the loaded data", () => {
    render(<DashboardPage />);

    expect(screen.getByText("Total OLTs")).toBeInTheDocument();
    expect(screen.getByText("Total ONTs")).toBeInTheDocument();
    expect(screen.getByText("50% online")).toBeInTheDocument();
    expect(screen.getByText("1 of 2 online")).toBeInTheDocument();
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
