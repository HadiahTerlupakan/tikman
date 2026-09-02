import { describe, expect, it } from "vitest";
import type { Odc, OdcFeed, Odp } from "@/domain/entities";
import { anchoredRoute, cableSegments, metersBetween } from "./cableSegments";

const olt = { id: "olt-1", latitude: -6.4, longitude: 107.0 };

const cabinet: Odc = {
  id: "odc-1",
  siteId: "site-1",
  code: "ODC-01",
  latitude: -6.41,
  longitude: 107.0,
  address: "",
  notes: "",
  feedCount: 1,
  odpCount: 1,
};

const box = (over: Partial<Odp> = {}): Odp => ({
  id: "odp-1",
  code: "ODP-01",
  portCount: 8,
  usedPorts: 0,
  latitude: -6.42,
  longitude: 107.0,
  address: "",
  notes: "",
  odcId: "odc-1",
  routeMeters: 0,
  ...over,
});

const feed = (over: Partial<OdcFeed> = {}): OdcFeed => ({
  id: "feed-1",
  odcId: "odc-1",
  oltId: "olt-1",
  slot: 1,
  portId: 1,
  splitterOutputs: 8,
  routeMeters: 0,
  ...over,
});

describe("metersBetween", () => {
  it("measures a degree of latitude the way the globe does", () => {
    expect(metersBetween({ lat: 0, lng: 0 }, { lat: 1, lng: 0 })).toBeCloseTo(
      111195,
      -3,
    );
  });
});

describe("cableSegments", () => {
  it("draws a feeder and a distribution cable from the topology alone", () => {
    const segments = cableSegments([olt], [cabinet], [box()], [feed()]);

    // No cable list anywhere: a feeder is a cabinet's feed, a distribution
    // cable is a box's parent link.
    expect(segments.map((s) => s.kind)).toEqual(["feeder", "distribution"]);
  });

  it("uses the straight line for a cable nobody has traced", () => {
    const [feeder] = cableSegments([olt], [cabinet], [], [feed()]);

    expect(feeder.traced).toBe(false);
    expect(feeder.path).toHaveLength(2);
    // A tenth of a degree of latitude, so a shade over 1.1 km.
    expect(feeder.meters).toBeGreaterThan(1000);
  });

  it("uses the traced path and its stored length when there is one", () => {
    const traced = [
      { lat: -6.4, lng: 107.0 },
      { lat: -6.405, lng: 107.01 },
      { lat: -6.41, lng: 107.0 },
    ];
    const [feeder] = cableSegments(
      [olt],
      [cabinet],
      [],
      [feed({ route: traced, routeMeters: 2500 })],
    );

    // The length the server measured over that path, not the gap between the
    // ends — the difference is the reason for tracing it.
    expect(feeder.traced).toBe(true);
    expect(feeder.path).toHaveLength(3);
    expect(feeder.meters).toBe(2500);
  });

  it("draws a box hanging straight off a PON port from the OLT", () => {
    const segments = cableSegments(
      [olt],
      [],
      [box({ odcId: undefined, oltId: "olt-1", slot: 1, portId: 4 })],
      [],
    );

    expect(segments).toHaveLength(1);
    expect(segments[0].path[0]).toEqual({ lat: -6.4, lng: 107.0 });
  });

  it("leaves out a cable whose ends are not both placed", () => {
    // A line drawn from a missing coordinate lands on the equator and claims a
    // cable runs there.
    expect(
      cableSegments([olt], [cabinet], [box({ latitude: undefined })], []),
    ).toEqual([]);
    expect(cableSegments([], [cabinet], [], [feed()])).toEqual([]);
  });

  it("leaves out a box whose parent it cannot find", () => {
    expect(
      cableSegments([olt], [], [box({ odcId: "odc-yang-hilang" })], []),
    ).toEqual([]);
  });

  it("ignores a stored path too short to be one", () => {
    // The server refuses these, but an older row could carry one, and drawing
    // a single point as a cable draws nothing while claiming it is traced.
    const [feeder] = cableSegments(
      [olt],
      [cabinet],
      [],
      [feed({ route: [{ lat: -6.4, lng: 107 }], routeMeters: 0 })],
    );

    expect(feeder.traced).toBe(false);
    expect(feeder.path).toHaveLength(2);
  });
});

describe("anchoredRoute", () => {
  const [feeder] = cableSegments([olt], [cabinet], [], [feed()]);

  it("keeps the cable's real ends around what was traced", () => {
    const route = anchoredRoute(feeder, [{ lat: -6.405, lng: 107.02 }]);

    // Otherwise a route clicked near the middle draws a cable floating short of
    // both the OLT and the cabinet.
    expect(route[0]).toEqual({ lat: -6.4, lng: 107.0 });
    expect(route[route.length - 1]).toEqual({ lat: -6.41, lng: 107.0 });
    expect(route).toHaveLength(3);
  });

  it("still yields a usable route when nothing was traced in between", () => {
    // Two points is the straight line, which is exactly what it should mean.
    expect(anchoredRoute(feeder, [])).toHaveLength(2);
  });
});
