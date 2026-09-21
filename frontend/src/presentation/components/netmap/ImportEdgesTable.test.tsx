import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ImportedEdge } from "@/domain/entities";
import { ImportEdgesTable } from "./ImportEdgesTable";

function edgeAt(row: number): ImportedEdge {
  return {
    row,
    edgeId: `E-${row}`,
    source: "ODC-01",
    target: "ODP-01",
    fiberType: "distribution",
    distance: 0,
    waypoints: null,
    notes: "",
    reason: "",
    conflict: false,
    conflictReason: "",
    unresolved: false,
    include: true,
  };
}

// Same measurement and reasoning as ImportNodesTable.test.tsx: unpaginated,
// every row re-renders on every keystroke anywhere in the table, costing
// 7,691-10,113 ms in real Chrome at 2,500 rows; pagination alone brings
// that to low tens of milliseconds by only mounting one page's rows. This
// asserts which rows actually reach the DOM, not that a `pagination` prop
// is merely set.
describe("ImportEdgesTable pagination", () => {
  it("renders only one page of rows, not the whole dataset", () => {
    const edges = Array.from({ length: 120 }, (_, i) => edgeAt(i + 1));

    render(<ImportEdgesTable edges={edges} onChange={vi.fn()} />);

    expect(screen.getByDisplayValue("E-1")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("E-51")).not.toBeInTheDocument();
  });
});
