import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import RoutersPage from "../RoutersPage";
import { UserRole } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";

const routers = vi.hoisted(() => ({ data: [] as Record<string, unknown>[] }));

const defaultRouters = [
  {
    id: "r-1",
    name: "border-1",
    host: "192.0.2.1",
    enabled: true,
    status: "up",
    apiStatus: "ok",
    cpuLoad: null,
    rosVersion: null,
    boardName: null,
    bgpEstablished: 3,
    bgpTotal: 4,
    silentInterfaces: 0,
    rackPosition: null,
  },
  {
    id: "r-2",
    name: "border-2",
    host: "192.0.2.2",
    enabled: true,
    status: "down",
    apiStatus: "ok",
    cpuLoad: null,
    rosVersion: null,
    boardName: null,
    bgpEstablished: 0,
    bgpTotal: 2,
    silentInterfaces: 0,
    rackPosition: null,
  },
];

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useMikrotikRouters: () => ({ isLoading: false, data: routers.data }),
  useRouterInterfaces: () => ({ data: [] }),
  useRouterBgpSessions: () => ({ data: [] }),
}));

vi.mock("@/presentation/components/routers/RouterFormModal", () => ({
  RouterFormModal: ({ router }: { router?: { name: string } }) => (
    <div>{router ? `editing ${router.name}` : "adding"}</div>
  ),
}));

vi.mock("@/presentation/components/routers/RouterStatusTag", () => ({
  RouterStatusTag: () => null,
}));

function renderAs(role: UserRole) {
  useAuthStore.setState({ user: { role } } as never);
  render(
    <MemoryRouter>
      <RoutersPage />
    </MemoryRouter>,
  );
}

describe("RoutersPage", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null } as never);
    routers.data = defaultRouters;
  });

  it("counts a BRAS's subscribers instead of its BGP sessions", () => {
    routers.data = [
      { ...routers.data[0], role: "border" },
      {
        ...routers.data[0],
        id: "r-3",
        name: "bras-1",
        status: "up",
        role: "bras",
        bgpEstablished: 0,
        bgpTotal: 5,
        bras: { sessions: 1842 },
      },
    ];
    renderAs(UserRole.ADMIN);
    expect(
      screen.getByText(
        "2 router · 2 terjangkau · BGP 3/4 established · 1.842 pelanggan online",
      ),
    ).toBeInTheDocument();
  });

  it("opens the edit form for the chosen router from the list", async () => {
    renderAs(UserRole.ADMIN);
    const [first] = screen.getAllByRole("button", { name: "Ubah" });
    await userEvent.click(first);
    expect(screen.getByText("editing border-1")).toBeInTheDocument();
  });

  it("offers an edit for every router to an admin", () => {
    renderAs(UserRole.ADMIN);
    expect(screen.getAllByRole("button", { name: "Ubah" })).toHaveLength(2);
  });

  it("sums up the rack above it", () => {
    renderAs(UserRole.TECHNICIAN);
    expect(
      screen.getByText(
        "2 router · 1 terjangkau, 1 tidak terjangkau · BGP 3/6 established",
      ),
    ).toBeInTheDocument();
  });

  it("offers no edit to a technician", () => {
    renderAs(UserRole.TECHNICIAN);
    expect(screen.queryByRole("button", { name: "Ubah" })).toBeNull();
  });
});
