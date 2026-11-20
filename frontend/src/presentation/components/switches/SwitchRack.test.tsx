import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ManagedSwitch, SwitchInterface } from "@/domain/entities";
import { SwitchRack } from "./SwitchRack";

const ports: SwitchInterface[] = [
  { name: "MEth0/0/0", kind: "rj" },
  { name: "10GE1/0/1", alias: "ke-olt" },
  { name: "10GE1/0/2", alias: "ke-pop", operStatus: "down" },
  { name: "Vlanif10", kind: "virtual" },
].map(
  (over) =>
    ({
      id: over.name,
      switchId: "s-1",
      alias: "",
      kind: "sfp",
      adminUp: true,
      operStatus: "up",
      speedMbps: 10000,
      lastInBps: 1e6,
      lastOutBps: 1e5,
      ...over,
    }) as SwitchInterface,
);

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchInterfaces: () => ({
    data: ports,
    isLoading: false,
    isError: false,
  }),
}));

const sw = (over: Partial<ManagedSwitch> = {}): ManagedSwitch =>
  ({
    id: "s-1",
    name: "SW-TEST",
    vendor: "huawei_ce",
    model: "CE6850-48S4Q-EI",
    enabled: true,
    status: "up",
    rackPosition: null,
    ...over,
  }) as ManagedSwitch;

describe("SwitchRack", () => {
  it("draws a CE6850 from its template with the ports' state", () => {
    const { container } = render(
      <SwitchRack switches={[sw()]} isAdmin={false} onEdit={vi.fn()} />,
    );

    expect(container.querySelectorAll("[data-port]")).toHaveLength(53);
    expect(container.querySelector('[data-port="Vlanif10"]')).toBeNull();
    expect(screen.getByText("1 port mati")).toBeTruthy();
    expect(screen.getAllByText("10GE1").length).toBeGreaterThan(0);
    expect(screen.queryByText("Ubah")).toBeNull();
  });

  it("leaves a blank for an empty rack position and lets an admin edit", async () => {
    const onEdit = vi.fn();
    render(
      <SwitchRack
        switches={[sw({ rackPosition: 2 })]}
        isAdmin
        onEdit={onEdit}
      />,
    );

    expect(screen.getAllByTestId("rack-blank")).toHaveLength(1);
    await userEvent.click(screen.getByText("Ubah"));
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ id: "s-1" }));
  });

  it("says an unreachable switch is unreachable", () => {
    render(
      <SwitchRack
        switches={[sw({ status: "down" })]}
        isAdmin={false}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Tidak terjangkau")).toBeTruthy();
  });
});
