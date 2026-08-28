import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OntOpticalTab } from "./OntOpticalTab";
import { rxSignalQuality, txSignalQuality } from "./signalQuality";

describe("rxSignalQuality", () => {
  // Checked against 30 minutes of this OLT's readings: 1178 land in Good,
  // 394 in Marginal and 150 in Weak, so the bands separate real populations
  // rather than putting almost everything in one.
  it("grades a receive level against the thresholds a technician works to", () => {
    expect(rxSignalQuality(-20).label).toBe("Good");
    expect(rxSignalQuality(-25).label).toBe("Good");
    expect(rxSignalQuality(-25.85).label).toBe("Marginal");
    expect(rxSignalQuality(-30).label).toBe("Weak");
  });

  // An ONU sitting metres from the OLT can be overdriven, which damages the
  // receiver and does not look like a fault from the dBm figure alone.
  it("flags a level that is too strong rather than calling it good", () => {
    expect(rxSignalQuality(-5).label).toBe("Too strong");
  });
});

describe("txSignalQuality", () => {
  it("accepts the normal transmit window and rejects outside it", () => {
    expect(txSignalQuality(2.4).label).toBe("Normal");
    expect(txSignalQuality(6).label).toBe("Out of range");
  });
});

describe("OntOpticalTab", () => {
  // The dBm figure is Ant Design's to render; what this component adds is the
  // verdict beside it, so that is what is asserted.
  it("puts a verdict beside each reading", () => {
    render(
      <OntOpticalTab
        metrics={{ rxPower: -25.85, txPower: 2.54, distance: 1004 }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Marginal")).toBeInTheDocument();
    expect(screen.getByText("Normal")).toBeInTheDocument();
  });

  // A dark fibre reports no power at all, which is not a reading of zero.
  it("says there is no signal rather than showing a number", () => {
    render(
      <OntOpticalTab
        metrics={{ rxPower: null, txPower: null }}
        isLoading={false}
      />,
    );

    expect(screen.getAllByText("No signal")).toHaveLength(2);
  });
});
