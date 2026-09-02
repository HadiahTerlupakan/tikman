import { describe, expect, it } from "vitest";
import type { Odp } from "@/domain/entities";
import { mappedPlant, odpFullness, odpPinColor } from "./plantMarkers";

const box = (over: Partial<Odp> = {}): Odp => ({
  id: "odp-1",
  code: "ODP-01",
  portCount: 8,
  usedPorts: 0,
  address: "",
  notes: "",
  latitude: -6.4,
  longitude: 106.8,
  ...over,
});

describe("odpFullness", () => {
  it("reads an empty box as room to spare", () => {
    expect(odpFullness(box({ usedPorts: 0 }))).toBe(0);
  });

  it("reads a full box as full", () => {
    expect(odpFullness(box({ usedPorts: 8 }))).toBe(1);
  });

  it("survives a box recorded with no ports", () => {
    // Nothing should divide by zero on a row someone typed badly, and a box
    // with no ports has no room by definition.
    expect(odpFullness(box({ portCount: 0, usedPorts: 0 }))).toBe(1);
  });

  it("does not exceed full when more subscribers are recorded than ports", () => {
    // The database refuses this, but an older row or an import could carry it,
    // and a fraction above one would colour outside the scale.
    expect(odpFullness(box({ portCount: 8, usedPorts: 12 }))).toBe(1);
  });
});

describe("odpPinColor", () => {
  it("separates a box with room from one without", () => {
    // The technician's question is answered by the pin alone, before any click.
    expect(odpPinColor(box({ usedPorts: 1 }))).not.toBe(
      odpPinColor(box({ usedPorts: 8 })),
    );
  });

  it("warns before a box is completely gone", () => {
    // A box down to its last port should not read the same as a half-empty one,
    // or the map only warns once it is too late to plan.
    expect(odpPinColor(box({ usedPorts: 4 }))).not.toBe(
      odpPinColor(box({ usedPorts: 7 })),
    );
  });
});

describe("mappedPlant", () => {
  it("keeps only what can actually be drawn", () => {
    const drawable = box({ id: "ok" });
    const halfPlaced = box({ id: "half", longitude: undefined });
    const unplaced = box({
      id: "none",
      latitude: undefined,
      longitude: undefined,
    });

    const pins = mappedPlant([drawable, halfPlaced, unplaced]);

    // One coordinate is not a location; drawing it would put the box on the
    // equator or the meridian and claim that is where it is.
    expect(pins.map((pin) => pin.id)).toEqual(["ok"]);
  });

  it("accepts nothing at all", () => {
    expect(mappedPlant(undefined)).toEqual([]);
  });
});
