import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UnconfiguredOnuTable } from "./UnconfiguredOnuTable";

const onu = {
  oltId: "o1",
  oltName: "Depok",
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
        dataSource={[
          {
            oltId: "o1",
            oltName: "Depok",
            slot: 4,
            port: 2,
            serialNumber: "ZTEGCAFFC2FD",
          },
        ]}
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

  it("names the OLT that detected each ONU when several are listed", () => {
    // The list merges every OLT's scan, so without this column an operator
    // cannot tell which site an ONU is waiting at.
    render(
      <UnconfiguredOnuTable
        dataSource={[onu, { ...onu, oltId: "o2", oltName: "Bekasi" }]}
        isLoading={false}
        showOlt
        onCopySerial={vi.fn()}
      />,
    );

    expect(screen.getByText("Depok")).toBeInTheDocument();
    expect(screen.getByText("Bekasi")).toBeInTheDocument();
  });

  it("drops the OLT column when the list is already filtered to one", () => {
    render(
      <UnconfiguredOnuTable
        dataSource={[onu]}
        isLoading={false}
        onCopySerial={vi.fn()}
      />,
    );

    expect(screen.queryByText("Depok")).not.toBeInTheDocument();
  });

  it("keeps rows from different OLTs apart even on an identical serial", () => {
    // Two OLTs can report the same ONU serial at the same PON position; a key
    // without the OLT would collapse them into one row.
    render(
      <UnconfiguredOnuTable
        dataSource={[onu, { ...onu, oltId: "o2", oltName: "Bekasi" }]}
        isLoading={false}
        showOlt
        onCopySerial={vi.fn()}
      />,
    );

    expect(screen.getAllByText("HWTCB403E8A0")).toHaveLength(2);
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
