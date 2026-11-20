import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchPortsTab } from "./SwitchPortsTab";
import { fixturePorts } from "./switchFixtures";

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchInterfaces: () => ({ data: fixturePorts, isLoading: false }),
}));

describe("SwitchPortsTab", () => {
  it("lists every port with its VLANs and alias", () => {
    render(<SwitchPortsTab switchId="s-1" />);
    expect(screen.getByText("ke-olt")).toBeTruthy();
    expect(screen.getByText(/10,20-22/)).toBeTruthy();
    expect(screen.getByText("Vlanif10")).toBeTruthy();
  });

  it("filters by search text", async () => {
    render(<SwitchPortsTab switchId="s-1" />);
    await userEvent.type(screen.getByPlaceholderText(/Cari port/), "olt");
    expect(screen.getByText("ke-olt")).toBeTruthy();
    expect(screen.queryByText("Vlanif10")).toBeNull();
  });

  it("opens the port detail on a row click", async () => {
    render(<SwitchPortsTab switchId="s-1" />);
    await userEvent.click(screen.getByText("10GE1/0/2"));
    expect(await screen.findByText("Error masuk / keluar")).toBeTruthy();
  });
});
