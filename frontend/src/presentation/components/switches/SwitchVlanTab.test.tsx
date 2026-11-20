import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { SwitchVlanTab } from "./SwitchVlanTab";

vi.mock("@/application/hooks/useSwitches", async () => {
  const f = await import("./switchFixtures");
  const idle = f.fixturePort({
    name: "10GE1/0/40",
    mode: "access",
    pvid: 1,
    operStatus: "down",
  });
  return {
    useSwitchInterfaces: () => ({
      data: [...f.fixturePorts, idle],
      isLoading: false,
    }),
    useSwitchVlans: () => ({ data: f.fixtureVlans, isLoading: false }),
  };
});

const matrix = () => screen.getByTestId("vlan-matrix");

describe("SwitchVlanTab", () => {
  it("names each VLAN in the matrix", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    expect(within(matrix()).getByText("pelanggan")).toBeTruthy();
  });

  it("draws T and U per VLAN row and port column", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    const cell = (vid: number, port: string) =>
      within(matrix()).getByTestId(`cell-${vid}-${port}`).textContent;

    expect(cell(10, "10GE1/0/1")).toBe("T");
    expect(cell(30, "10GE1/0/1")).toBe("");
    expect(cell(30, "10GE1/0/2")).toBe("U");
    expect(cell(10, "10GE1/0/3")).toBe("U");
    expect(cell(30, "10GE1/0/3")).toBe("T");
    expect(within(matrix()).queryByText("Vlanif10")).toBeNull();
  });

  it("hides untouched ports until asked for", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    expect(screen.queryByTestId("cell-10-10GE1/0/40")).toBeNull();

    fireEvent.click(screen.getByRole("switch"));
    expect(screen.getByTestId("cell-10-10GE1/0/40")).toBeTruthy();
  });

  it("filters VLAN rows by id or name", () => {
    render(<SwitchVlanTab switchId="s-1" />);
    fireEvent.change(screen.getByPlaceholderText("Cari VLAN atau nama"), {
      target: { value: "olt" },
    });
    expect(screen.getByTestId("cell-30-10GE1/0/1")).toBeTruthy();
    expect(screen.queryByTestId("cell-10-10GE1/0/1")).toBeNull();

    fireEvent.change(screen.getByPlaceholderText("Cari VLAN atau nama"), {
      target: { value: "999" },
    });
    expect(screen.getByText("Tidak ada VLAN yang cocok")).toBeTruthy();
  });
});
