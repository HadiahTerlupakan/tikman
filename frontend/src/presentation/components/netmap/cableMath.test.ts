import { describe, expect, it } from "vitest";
import type { MappingNode } from "@/domain/entities";
import { edgePath, formatMeters, metersAlong } from "./cableMath";

describe("metersAlong", () => {
  // The length shown has to be the cable that was pulled, not the straight line
  // between its ends — that is the whole reason waypoints are stored.
  it("measures the path drawn, not the line between its ends", () => {
    const straight = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    const detoured = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.2, lng: 106.81 },
      { lat: -6.21, lng: 106.81 },
      { lat: -6.21, lng: 106.8 },
    ]);

    expect(detoured).toBeGreaterThan(straight);
  });

  it("is zero for a path that goes nowhere", () => {
    expect(metersAlong([])).toBe(0);
    expect(metersAlong([{ lat: -6.2, lng: 106.8 }])).toBe(0);
  });

  it("measures about a kilometre for a hundredth of a degree of latitude", () => {
    const m = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    expect(m).toBeGreaterThan(1050);
    expect(m).toBeLessThan(1160);
  });
});

describe("formatMeters", () => {
  it("reads in metres under a kilometre and in kilometres above it", () => {
    expect(formatMeters(240)).toBe("240 m");
    expect(formatMeters(1250)).toBe("1,25 km");
  });
});

describe("edgePath", () => {
  const source: MappingNode = {
    nodeId: "ODC-01",
    type: "odc",
    name: "ODC Satu",
    latitude: -6.2,
    longitude: 106.8,
    capacity: 0,
    splitter: "",
    pppoe: "",
    serialNumber: "",
    notes: "",
  };
  const target: MappingNode = {
    ...source,
    nodeId: "ODP-01",
    name: "ODP Satu",
    latitude: -6.21,
    longitude: 106.81,
  };
  const nodesById = new Map([
    [source.nodeId, source],
    [target.nodeId, target],
  ]);

  // This walk has to serve both a saved edge (MapCanvas, naming its ends
  // through a MappingEdge) and a cable that is only a pending save
  // (NetworkMapPage, naming them directly) — two separate copies of it is
  // exactly how the branch shipped a straight cable that measured 0 metres.
  it("spans from the source, through every traced corner, to the target", () => {
    const path = edgePath(
      {
        source: "ODC-01",
        target: "ODP-01",
        waypoints: [{ lat: -6.205, lng: 106.805 }],
      },
      nodesById,
    );

    expect(path).toEqual([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.205, lng: 106.805 },
      { lat: -6.21, lng: 106.81 },
    ]);
  });

  it("is skipped when the target has not been named yet", () => {
    const path = edgePath(
      { source: "ODC-01", target: "GHOST-404", waypoints: null },
      nodesById,
    );

    expect(path).toBeUndefined();
  });

  it("is skipped when the source has been removed from the map", () => {
    const path = edgePath(
      { source: "GHOST-404", target: "ODP-01", waypoints: null },
      nodesById,
    );

    expect(path).toBeUndefined();
  });
});
