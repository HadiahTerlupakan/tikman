import { describe, expect, it } from "vitest";
import type { Olt } from "@/domain/entities";
import { mappedOlts, unmappedOlts } from "./oltMapFilters";

const olt = (overrides: Partial<Olt> & { id: string }): Olt =>
  ({ name: "OLT", ...overrides }) as Olt;

describe("oltMapFilters", () => {
  it("keeps only the OLTs that carry both coordinates", () => {
    const olts = [
      olt({ id: "a", latitude: -6.4, longitude: 106.8 }),
      olt({ id: "b" }),
    ];

    expect(mappedOlts(olts).map((o) => o.id)).toEqual(["a"]);
    expect(unmappedOlts(olts).map((o) => o.id)).toEqual(["b"]);
  });

  it("partitions exactly, so nothing is lost or counted twice", () => {
    // A half-set coordinate is the case that could fall through both filters
    // or land in both, leaving an OLT invisible on the map and absent from the
    // panel that exists to name it.
    const olts = [
      olt({ id: "a", latitude: -6.4, longitude: 106.8 }),
      olt({ id: "b", latitude: -6.4 }),
      olt({ id: "c", longitude: 106.8 }),
      olt({ id: "d" }),
    ];

    const mapped = mappedOlts(olts).map((o) => o.id);
    const unmapped = unmappedOlts(olts).map((o) => o.id);

    expect(mapped).toEqual(["a"]);
    expect(unmapped).toEqual(["b", "c", "d"]);
    expect(mapped.length + unmapped.length).toBe(olts.length);
    expect(mapped.filter((id) => unmapped.includes(id))).toEqual([]);
  });

  it("treats a genuine zero as a coordinate, not as absent", () => {
    const atNullIsland = olt({ id: "z", latitude: 0, longitude: 0 });

    expect(mappedOlts([atNullIsland])).toHaveLength(1);
  });

  it("returns empty lists rather than throwing on no data", () => {
    expect(mappedOlts(undefined)).toEqual([]);
    expect(unmappedOlts(undefined)).toEqual([]);
  });
});
