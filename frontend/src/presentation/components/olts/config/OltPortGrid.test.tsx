import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { OltPort } from "@/domain/entities";
import { OltPortGrid } from "./OltPortGrid";

function port(overrides: Partial<OltPort> = {}): OltPort {
  return {
    ifIndex: 1,
    name: "gpon_1/3/1",
    kind: "pon",
    rack: 1,
    slot: 3,
    port: 1,
    adminUp: true,
    operUp: true,
    adminStatus: 1,
    operStatus: 1,
    ...overrides,
  };
}

describe("OltPortGrid", () => {
  it("draws one card per slot and counts the ports that are up", () => {
    render(
      <OltPortGrid
        ports={[
          port({ ifIndex: 1, name: "gpon_1/3/1", port: 1 }),
          port({ ifIndex: 2, name: "gpon_1/3/2", port: 2, operUp: false }),
          port({ ifIndex: 3, name: "gpon_1/4/1", slot: 4, port: 1 }),
        ]}
        kind="pon"
        cardLabel={(slot) => `Card ${slot}`}
        emptyText="nothing"
      />,
    );

    expect(screen.getByText("Card 3")).toBeInTheDocument();
    expect(screen.getByText("Card 4")).toBeInTheDocument();
    expect(screen.getByText("1 / 2 up")).toBeInTheDocument();
    expect(screen.getByTestId("port-gpon_1/3/2")).toBeInTheDocument();
  });

  it("says so when the poll reported no port of this kind", () => {
    render(
      <OltPortGrid
        ports={[port()]}
        kind="uplink"
        cardLabel={(slot) => `Slot ${slot}`}
        emptyText="No uplink ports reported by the last poll"
      />,
    );

    expect(
      screen.getByText("No uplink ports reported by the last poll"),
    ).toBeInTheDocument();
  });
});
