import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchNeighborsTab } from "./SwitchNeighborsTab";
import { fixtureNeighbors } from "./switchFixtures";

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchNeighbors: () => ({ data: fixtureNeighbors, isLoading: false }),
}));

describe("SwitchNeighborsTab", () => {
  it("splits LLDP neighbours from MAC addresses", () => {
    render(<SwitchNeighborsTab switchId="s-1" />);
    expect(screen.getByText("OLT-TEST")).toBeTruthy();
    expect(screen.getByText("00:00:5e:00:53:01")).toBeTruthy();
  });

  it("searches the MAC table", async () => {
    render(<SwitchNeighborsTab switchId="s-1" />);
    await userEvent.type(screen.getByPlaceholderText(/Cari MAC/), "zzzz");
    expect(screen.queryByText("00:00:5e:00:53:01")).toBeNull();
    expect(screen.getByText("OLT-TEST")).toBeTruthy();
  });
});
