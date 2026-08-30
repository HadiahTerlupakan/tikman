import { describe, expect, it } from "vitest";
import type { Site } from "@/domain/entities";
import { mappedSites, unmappedSites } from "./siteMapFilters";

function site(overrides: Partial<Site> & { id: string }): Site {
  return {
    name: overrides.id,
    location: "",
    description: "",
    oltCount: 0,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

describe("site map filters", () => {
  const depok = site({ id: "depok", latitude: -6.4025, longitude: 106.7942 });
  const bekasi = site({ id: "bekasi", latitude: -6.2383, longitude: 106.9756 });
  const noCoordinates = site({ id: "gudang" });
  const halfCoordinates = site({ id: "tower", latitude: -6.3 });
  const all = [depok, noCoordinates, bekasi, halfCoordinates];

  it("draws only the sites that carry both coordinates", () => {
    expect(mappedSites(all).map((s) => s.id)).toEqual(["depok", "bekasi"]);
  });

  it("accounts for every site the map cannot place", () => {
    expect(unmappedSites(all).map((s) => s.id)).toEqual(["gudang", "tower"]);
  });

  it("partitions the list exactly, so no site is lost or counted twice", () => {
    // A site with one coordinate is the case that would otherwise slip through
    // both filters and vanish from the page entirely.
    const mapped = mappedSites(all);
    const unmapped = unmappedSites(all);

    expect(mapped.length + unmapped.length).toBe(all.length);
    expect(
      mapped.filter((m) => unmapped.some((u) => u.id === m.id)),
    ).toHaveLength(0);
  });

  it("treats a missing list as no sites rather than throwing", () => {
    expect(mappedSites(undefined)).toEqual([]);
    expect(unmappedSites(undefined)).toEqual([]);
  });
});
