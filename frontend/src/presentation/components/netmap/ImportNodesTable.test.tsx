import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ImportedNode } from "@/domain/entities";
import { ImportNodesTable } from "./ImportNodesTable";

function nodeAt(row: number): ImportedNode {
  return {
    row,
    nodeId: `ODP-${row}`,
    type: "odp",
    name: `ODP ${row}`,
    latitude: -6.2,
    longitude: 106.8,
    capacity: 0,
    splitter: "",
    pppoe: "",
    serialNumber: "",
    notes: "",
    reason: "",
    conflict: false,
    conflictReason: "",
    blocked: false,
    blockedReason: "",
    include: true,
  };
}

// Measured in real Chrome at 2,500 rows: unpaginated, a single keystroke in
// any field cost 7,691-10,113 ms (every row re-rendering on every change)
// and first paint cost 9,338 ms; paginated to 50 rows per page, both fell to
// low tens of milliseconds (220,040 DOM nodes down to 4,530). jsdom cannot
// reproduce the timing here - it OOMs at that row count, one such render
// measured at 33 minutes - but which rows actually reach the DOM is exactly
// what pagination controls, and jsdom's DOM output is otherwise faithful.
// Asserting the `pagination` prop is merely set would be the test theatre
// this feature has already had to fix three times; this asserts the thing
// that actually explains the fix.
describe("ImportNodesTable pagination", () => {
  it("renders only one page of rows, not the whole dataset", () => {
    const nodes = Array.from({ length: 120 }, (_, i) => nodeAt(i + 1));

    render(<ImportNodesTable nodes={nodes} onChange={vi.fn()} />);

    expect(screen.getByDisplayValue("ODP-1")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("ODP-51")).not.toBeInTheDocument();
  });
});
