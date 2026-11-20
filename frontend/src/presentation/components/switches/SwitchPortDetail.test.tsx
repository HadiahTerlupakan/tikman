import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { SwitchPortDetail } from "./SwitchPortDetail";
import { fixturePort } from "./switchFixtures";

describe("SwitchPortDetail", () => {
  it("shows a trunk's allowed VLANs one tag per entry, alias unquoted", () => {
    render(
      <SwitchPortDetail
        port={fixturePort({
          name: "40GE1/0/1",
          alias: '"TO-CCR2216"',
          operStatus: "down",
          mode: "trunk",
          pvid: 1,
          allowedVlans: "10,202,1118-1119",
        })}
      />,
    );
    expect(screen.getByText("Mati")).toBeTruthy();
    expect(screen.getByText("TO-CCR2216")).toBeTruthy();
    expect(screen.getByText("1118-1119")).toBeTruthy();
    expect(screen.getByText("202")).toBeTruthy();
  });

  it("leaves the VLAN list out for an access port", () => {
    render(
      <SwitchPortDetail
        port={fixturePort({ name: "10GE1/0/2", mode: "access", pvid: 2505 })}
      />,
    );
    expect(screen.getByText("2505")).toBeTruthy();
    expect(screen.queryByText("VLAN yang dilewatkan")).toBeNull();
  });
});
