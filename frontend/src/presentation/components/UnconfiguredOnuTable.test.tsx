import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UnconfiguredOnuTable } from "./UnconfiguredOnuTable";

const onu = {
  slot: 3,
  port: 1,
  serialNumber: "HWTCB403E8A0",
  deviceType: "HG8245H5",
  softwareVersion: "V5R019C00S105",
};

describe("UnconfiguredOnuTable", () => {
  it("renders the PON position and device details", () => {
    render(
      <UnconfiguredOnuTable
        dataSource={[onu]}
        isLoading={false}
        onCopySerial={vi.fn()}
      />,
    );

    expect(screen.getByText("3/1")).toBeInTheDocument();
    expect(screen.getByText("HWTCB403E8A0")).toBeInTheDocument();
    expect(screen.getByText("HG8245H5")).toBeInTheDocument();
    expect(screen.getByText("V5R019C00S105")).toBeInTheDocument();
  });

  it("falls back to a dash when the OLT reports no model or firmware", () => {
    render(
      <UnconfiguredOnuTable
        dataSource={[{ slot: 4, port: 2, serialNumber: "ZTEGCAFFC2FD" }]}
        isLoading={false}
        onCopySerial={vi.fn()}
      />,
    );

    expect(screen.getAllByText("-")).toHaveLength(2);
  });

  it("copies the serial number of the clicked row", async () => {
    const onCopySerial = vi.fn();
    render(
      <UnconfiguredOnuTable
        dataSource={[onu]}
        isLoading={false}
        onCopySerial={onCopySerial}
      />,
    );

    await userEvent.click(
      screen.getByLabelText("Copy serial number HWTCB403E8A0"),
    );

    expect(onCopySerial).toHaveBeenCalledWith("HWTCB403E8A0");
  });

  it("tells the operator when nothing is awaiting provisioning", () => {
    render(
      <UnconfiguredOnuTable
        dataSource={[]}
        isLoading={false}
        onCopySerial={vi.fn()}
      />,
    );

    expect(
      screen.getByText("No unconfigured ONUs detected"),
    ).toBeInTheDocument();
  });
});
