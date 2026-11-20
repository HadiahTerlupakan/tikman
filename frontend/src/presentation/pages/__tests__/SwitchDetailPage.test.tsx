import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { UserRole } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import SwitchDetailPage from "../SwitchDetailPage";
import { fixtureSwitch } from "@/presentation/components/switches/switchFixtures";

const switches = vi.hoisted(() => ({ data: [] as unknown[] }));

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitches: () => ({ data: switches.data, isLoading: false }),
  useTrustSwitchHostKey: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchBackupNow: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchBackups: () => ({ data: [], isLoading: false }),
  useSwitchBackup: () => ({ data: undefined }),
  useSwitchBackupDiff: () => ({ data: undefined }),
}));
vi.mock("@/presentation/components/switches/SwitchFaceplate", () => ({
  SwitchFaceplate: () => <div>faceplate</div>,
}));
vi.mock("@/presentation/components/switches/SwitchPortsTab", () => ({
  SwitchPortsTab: () => <div>ports tab</div>,
}));

function renderAs(role: UserRole, id = "s-1") {
  useAuthStore.setState({ user: { role } } as never);
  render(
    <MemoryRouter initialEntries={[`/switches/${id}`]}>
      <Routes>
        <Route path="/switches/:id" element={<SwitchDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("SwitchDetailPage", () => {
  beforeEach(() => {
    switches.data = [fixtureSwitch()];
  });

  it("opens on the front panel", () => {
    renderAs(UserRole.VIEWER);
    expect(screen.getByText("faceplate")).toBeTruthy();
    expect(screen.getByText("SW-TEST")).toBeTruthy();
  });

  it("says when the switch does not exist", () => {
    renderAs(UserRole.VIEWER, "nope");
    expect(screen.getByText("Switch tidak ditemukan")).toBeTruthy();
  });

  it("gives the backup button to a technician but not a viewer", async () => {
    renderAs(UserRole.TECHNICIAN);
    await userEvent.click(screen.getByRole("tab", { name: "Config" }));
    expect(
      screen.getByRole("button", { name: "Backup sekarang" }),
    ).toBeTruthy();
  });

  it("hides the backup button from a viewer", async () => {
    renderAs(UserRole.VIEWER);
    await userEvent.click(screen.getByRole("tab", { name: "Config" }));
    expect(
      screen.queryByRole("button", { name: "Backup sekarang" }),
    ).toBeNull();
  });

  it("raises the host key banner and lets only an admin trust", () => {
    switches.data = [
      fixtureSwitch({ hostKeyChanged: true, presentedFingerprint: "FP+new" }),
    ];
    renderAs(UserRole.ADMIN);
    expect(screen.getByText("FP+new")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Percayai" })).toBeTruthy();
  });
});
