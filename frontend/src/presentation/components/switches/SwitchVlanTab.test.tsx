import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SwitchVlanTab } from "./SwitchVlanTab";
import { fixturePorts, fixtureVlans } from "./switchFixtures";

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchInterfaces: () => ({ data: fixturePorts, isLoading: false }),
  useSwitchVlans: () => ({ data: fixtureVlans, isLoading: false }),
}));

describe("SwitchVlanTab", () => {
  it("lists the VLANs", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    expect(screen.getAllByText("pelanggan").length).toBeGreaterThan(0);
  });

  it("draws T and U per VLAN row and port column", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    const matrix = screen.getByTestId("vlan-matrix");
    const cell = (vid: number, port: string) =>
      within(matrix).getByTestId(`cell-${vid}-${port}`).textContent;

    expect(cell(10, "10GE1/0/1")).toBe("T");
    expect(cell(30, "10GE1/0/1")).toBe("");
    expect(cell(30, "10GE1/0/2")).toBe("U");
    expect(cell(10, "10GE1/0/3")).toBe("U");
    expect(cell(30, "10GE1/0/3")).toBe("T");
    expect(within(matrix).queryByText("Vlanif10")).toBeNull();
  });
});
