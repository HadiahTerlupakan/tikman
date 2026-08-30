import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { WeakSignalList } from "./WeakSignalList";

describe("WeakSignalList", () => {
  it("explains an empty list instead of showing a blank panel", () => {
    render(<WeakSignalList signals={[]} />);

    expect(
      screen.getByText("No online ONT is reporting an optical reading."),
    ).toBeInTheDocument();
  });

  it("shows the reading and where to find the ONT", () => {
    render(
      <WeakSignalList
        signals={[
          {
            id: "1",
            name: "Rumah Pak Budi",
            serialNumber: "ZTEGC0FFEE01",
            oltName: "Depok",
            rxPower: -28.42,
          },
        ]}
      />,
    );

    expect(screen.getByText("Rumah Pak Budi")).toBeInTheDocument();
    expect(screen.getByText("Depok")).toBeInTheDocument();
    // One decimal: SNMP reports more precision than the optics justify.
    expect(screen.getByText("-28.4 dBm")).toBeInTheDocument();
  });
});
