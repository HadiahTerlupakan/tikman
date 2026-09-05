import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { OntTable } from "./OntTable";
import { OntStatus, type Ont } from "@/domain/entities";

const ont = (over: Partial<Ont> = {}): Ont => ({
  id: "o1",
  oltId: "olt1",
  oltName: "Cariu",
  slot: 9,
  portId: 1,
  ontId: 45,
  serialNumber: "ZTEG12345678",
  name: "Budi",
  description: "",
  status: OntStatus.ONLINE,
  lastSeenAt: null,
  createdAt: "2026-09-05T00:00:00Z",
  updatedAt: "2026-09-05T00:00:00Z",
  ...over,
});

function renderTable(rows: Ont[]) {
  render(
    <OntTable
      dataSource={rows}
      isLoading={false}
      page={1}
      total={rows.length}
      pageSize={10}
      onPageChange={() => {}}
      onPageSizeChange={() => {}}
      onViewDetail={() => {}}
      onDelete={() => {}}
    />,
  );
}

describe("OntTable position column", () => {
  // Port numbers repeat across cards — on 2026-09-05 production had PON port 1
  // on cards 2, 3, 8 and 9 at once — so a row naming only the port does not
  // say which ONU it is.
  it("names the card the port belongs to", () => {
    renderTable([ont()]);

    expect(screen.getByText("Kartu:")).toBeInTheDocument();
    expect(screen.getByText("9")).toBeInTheDocument();
  });

  // The column is nullable, and discovery only writes what a walk reports.
  // An empty label reads as "card zero"; no line reads as "not known".
  it("says nothing about the card when the OLT never reported one", () => {
    renderTable([ont({ slot: undefined })]);

    expect(screen.queryByText("Kartu:")).toBeNull();
    expect(screen.getByText("PON Port:")).toBeInTheDocument();
    expect(screen.getByText("ONT ID:")).toBeInTheDocument();
  });
});
