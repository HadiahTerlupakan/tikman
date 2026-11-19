import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { RouterDetailPage } from "../RouterDetailPage";

const routers = vi.hoisted(() => ({
  data: [] as Record<string, unknown>[],
}));

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useMikrotikRouters: () => ({ isLoading: false, data: routers.data }),
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
vi.mock("@/presentation/components/routers/bras/BrasSummaryTab", () => ({
  BrasSummaryTab: () => <div>bras summary</div>,
}));
vi.mock("@/presentation/components/routers/bras/BrasTrafficTab", () => ({
  BrasTrafficTab: () => null,
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
  beforeEach(() => {
    routers.data = [
      {
        id: "r-1",
        name: "border-1",
        enabled: true,
        status: "up",
        role: "border",
      },
    ];
  });

  it("keeps the six tabs of a border router", () => {
    renderPage();
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Panel depan",
      "Upstream & BGP",
      "Jalur BGP",
      "Tujuan Trafik",
      "Interface & VLAN",
      "Router",
    ]);
  });

  it("gives a BRAS no upstream tab", () => {
    routers.data = [{ ...routers.data[0], name: "bras-1", role: "bras" }];
    renderPage();
    expect(screen.queryByRole("tab", { name: "Upstream & BGP" })).toBeNull();
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Ringkasan",
      "Panel depan",
      "Trafik",
      "Interface & VLAN",
      "Router",
    ]);
  });

  it("gives a router of another role only its ports and settings", () => {
    routers.data = [{ ...routers.data[0], name: "core-1", role: "other" }];
    renderPage();
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Panel depan",
      "Interface & VLAN",
      "Router",
    ]);
  });

  it("tapes the role beside the router's name", () => {
    routers.data = [{ ...routers.data[0], name: "bras-1", role: "bras" }];
    renderPage();
    expect(document.querySelector(".fp-roletape-bras")?.textContent).toBe(
      "BRAS",
    );
  });

  it("opens a BRAS on its summary", () => {
    routers.data = [{ ...routers.data[0], name: "bras-1", role: "bras" }];
    renderPage();
    expect(
      screen.getByRole("tab", { name: "Ringkasan", selected: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("bras summary")).toBeVisible();
  });

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
