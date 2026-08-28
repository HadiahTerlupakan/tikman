import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OltSpeedTable } from "./OltSpeedTable";
import { formatBandwidth } from "./oltSystem";

describe("formatBandwidth", () => {
  // The OLT reports kbps and the profile name is only a label: "1G" grants
  // 1024000 kbps, so the figure has to come from the device, not the name.
  it("converts kbps to Mbps", () => {
    expect(formatBandwidth(1024000)).toBe("1024 Mbps");
    expect(formatBandwidth(10000)).toBe("10 Mbps");
  });

  it("shows a dash where the profile grants nothing", () => {
    expect(formatBandwidth(0)).toBe("—");
  });
});

describe("OltSpeedTable", () => {
  it("shows each profile's maximum bandwidth", () => {
    render(
      <OltSpeedTable
        profiles={[
          {
            name: "1G",
            type: 3,
            fixedBwKbps: 0,
            assuredBwKbps: 512,
            maxBwKbps: 1024000,
          },
        ]}
      />,
    );

    expect(screen.getByText("1G")).toBeInTheDocument();
    expect(screen.getByText("1024 Mbps")).toBeInTheDocument();
  });
});
