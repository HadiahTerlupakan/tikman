import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { RouterDetailPage } from "../RouterDetailPage";

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useMikrotikRouters: () => ({
    isLoading: false,
    data: [{ id: "r-1", name: "border-1", enabled: true, status: "up" }],
  }),
}));

vi.mock("@/presentation/components/routers/faceplate/RouterFaceplate", () => ({
  RouterFaceplate: () => <div>faceplate</div>,
}));
vi.mock("@/presentation/components/routers/UpstreamTab", () => ({
  UpstreamTab: () => <div>upstream tab</div>,
}));
vi.mock("@/presentation/components/routers/BgpPathsTab", () => ({
  BgpPathsTab: () => null,
}));
vi.mock("@/presentation/components/routers/FlowDestinationsTab", () => ({
  FlowDestinationsTab: () => null,
}));
vi.mock("@/presentation/components/routers/InterfacesTab", () => ({
  InterfacesTab: () => null,
}));
vi.mock("@/presentation/components/routers/RouterInfoTab", () => ({
  RouterInfoTab: () => null,
}));
vi.mock("@/presentation/components/routers/RouterStatusTag", () => ({
  RouterStatusTag: () => null,
}));

function renderPage() {
  render(
    <MemoryRouter initialEntries={["/routers/r-1"]}>
      <Routes>
        <Route path="/routers/:id" element={<RouterDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("RouterDetailPage", () => {
  it("opens on the front panel, in a tab of its own", () => {
    renderPage();
    expect(
      screen.getByRole("tab", { name: "Panel depan", selected: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("faceplate")).toBeInTheDocument();
    expect(screen.queryByText("upstream tab")).toBeNull();
  });

  it("leaves the front panel out of the other tabs", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("tab", { name: "Upstream & BGP" }));
    expect(screen.getByText("upstream tab")).toBeVisible();
    expect(screen.getByText("faceplate")).not.toBeVisible();
  });
});
