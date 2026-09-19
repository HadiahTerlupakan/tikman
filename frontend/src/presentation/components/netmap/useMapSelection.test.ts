import { describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { useMapSelection } from "./useMapSelection";

const node: MappingNode = {
  nodeId: "ODP-01",
  type: "odp",
  name: "ODP Satu",
  latitude: -6.2,
  longitude: 106.8,
  capacity: 8,
  splitter: "",
  pppoe: "",
  serialNumber: "",
  notes: "",
};

const edge: MappingEdge = {
  edgeId: "ODC-01--ODP-01",
  source: "ODC-01",
  target: "ODP-01",
  fiberType: "distribution",
  distance: 120,
  waypoints: [],
  notes: "",
};

describe("useMapSelection", () => {
  it("opens with no popup selected", () => {
    const { result } = renderHook(() => useMapSelection());

    expect(result.current.node).toBeUndefined();
    expect(result.current.edge).toBeUndefined();
  });

  it("selects a node for its popup", () => {
    const { result } = renderHook(() => useMapSelection());

    act(() => result.current.selectNode(node));

    expect(result.current.node).toBe(node);
  });

  // Only one InfoWindow makes sense on the map at a time; selecting a node
  // while a cable's popup is open must replace it, not stack alongside it.
  it("closes a selected cable's popup when a node is selected instead", () => {
    const { result } = renderHook(() => useMapSelection());

    act(() => result.current.selectEdge(edge));
    act(() => result.current.selectNode(node));

    expect(result.current.edge).toBeUndefined();
    expect(result.current.node).toBe(node);
  });

  it("selects a cable for its popup", () => {
    const { result } = renderHook(() => useMapSelection());

    act(() => result.current.selectEdge(edge));

    expect(result.current.edge).toBe(edge);
  });

  it("closes a selected node's popup when a cable is selected instead", () => {
    const { result } = renderHook(() => useMapSelection());

    act(() => result.current.selectNode(node));
    act(() => result.current.selectEdge(edge));

    expect(result.current.node).toBeUndefined();
    expect(result.current.edge).toBe(edge);
  });

  it("clears whichever popup is open", () => {
    const { result } = renderHook(() => useMapSelection());

    act(() => result.current.selectNode(node));
    act(() => result.current.clear());

    expect(result.current.node).toBeUndefined();
    expect(result.current.edge).toBeUndefined();
  });
});
