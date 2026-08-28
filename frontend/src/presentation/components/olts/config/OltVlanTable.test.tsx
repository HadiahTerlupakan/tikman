import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OltVlanTable } from "./OltVlanTable";

describe("OltVlanTable", () => {
  // A VLAN carried tagged on an uplink is a trunk member and one carried
  // untagged is an access port; putting both in one column loses that.
  it("separates tagged from untagged ports", () => {
    render(
      <OltVlanTable
        vlans={[
          {
            vlanId: 1,
            name: "VLAN0001",
            ports: [
              { slot: 10, port: 1, tagged: false },
              { slot: 10, port: 5, tagged: true },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByText("1/10/1")).toBeInTheDocument();
    expect(screen.getByText("1/10/5")).toBeInTheDocument();
  });

  // A device that does not publish the membership column must not read as a
  // VLAN present on no port at all.
  it("says so when membership was not reported", () => {
    render(<OltVlanTable vlans={[{ vlanId: 200, name: "V200", ports: [] }]} />);

    expect(screen.getAllByText("not reported")).toHaveLength(2);
  });
});
