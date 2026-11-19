import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import RoutersPage from "../RoutersPage";
import { UserRole } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useMikrotikRouters: () => ({
    isLoading: false,
    data: [
      {
        id: "r-1",
        name: "border-1",
        host: "192.0.2.1",
        apiStatus: "ok",
        cpuLoad: null,
        rosVersion: null,
        bgpEstablished: 0,
        bgpTotal: 0,
      },
    ],
  }),
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
  beforeEach(() => useAuthStore.setState({ user: null } as never));

  it("opens the edit form for the chosen router from the list", async () => {
    renderAs(UserRole.ADMIN);
    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));
    expect(screen.getByText("editing border-1")).toBeInTheDocument();
  });

  it("offers no edit to a technician", () => {
    renderAs(UserRole.TECHNICIAN);
    expect(screen.queryByRole("button", { name: "Ubah" })).toBeNull();
  });
});
