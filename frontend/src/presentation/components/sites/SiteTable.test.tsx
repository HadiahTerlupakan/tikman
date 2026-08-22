import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SiteTable } from "./SiteTable";
import type { Site } from "@/domain/entities";

const SITES: Site[] = [
  {
    id: "site-1",
    name: "Cariu",
    location: "Cariu",
    description: "Cariu",
    oltCount: 3,
    createdAt: "2026-08-15T12:52:46.458Z",
    updatedAt: "2026-08-15T12:52:46.458Z",
  },
  {
    id: "site-2",
    name: "Empty",
    location: "Nowhere",
    description: "",
    oltCount: 0,
    createdAt: "2026-08-15T12:52:46.458Z",
    updatedAt: "2026-08-15T12:52:46.458Z",
  },
];

function renderTable() {
  return render(
    <SiteTable
      sites={SITES}
      loading={false}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
    />,
  );
}

function cellTexts(rowName: string) {
  const row = screen.getByText(rowName).closest("tr");
  return Array.from(row?.querySelectorAll("td") || []).map(
    (td) => td.textContent,
  );
}

describe("SiteTable", () => {
  it("renders the OLT count as plain text, not a floating badge", () => {
    const { container } = renderTable();

    // A Badge renders the value inside <sup>, which reads as a notification dot
    // and merges with neighbouring text when copied out of the table.
    expect(container.querySelector("sup")).toBeNull();
    expect(cellTexts("Cariu")).toContain("3");
  });

  it("shows a zero count instead of leaving the cell blank", () => {
    renderTable();

    expect(cellTexts("Empty")).toContain("0");
  });
});
