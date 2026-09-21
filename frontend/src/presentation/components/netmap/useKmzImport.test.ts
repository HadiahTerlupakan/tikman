import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useKmzImport } from "./useKmzImport";

vi.mock("@/application/hooks", () => ({
  usePreviewImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCommitImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

// updateNode/updateEdge are what ImportNodesTable/ImportEdgesTable receive
// as `onChange`, threaded straight into every column's render function. A
// new function identity on every render of ImportKmzModal (which happens on
// every keystroke, since typing calls setPreview) forces antd's Table to
// treat every row as changed, even the ones whose data did not change -
// measured in a real browser at 2,500 rows: 7,691-10,113 ms per keystroke.
// Pagination already bounds this to one page's worth of rows; this is the
// other half - the callback these rows are keyed to should not itself be a
// new reference on every unrelated re-render.
describe("useKmzImport callback stability", () => {
  it("keeps the same updateNode and updateEdge identity across re-renders", () => {
    const { result, rerender } = renderHook(() => useKmzImport());
    const firstUpdateNode = result.current.updateNode;
    const firstUpdateEdge = result.current.updateEdge;

    rerender();

    expect(result.current.updateNode).toBe(firstUpdateNode);
    expect(result.current.updateEdge).toBe(firstUpdateEdge);
  });
});
