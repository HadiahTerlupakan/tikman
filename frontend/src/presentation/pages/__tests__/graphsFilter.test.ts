import { describe, it, expect } from "vitest";
import { filterOntsByQuery } from "../graphsFilter";
import { Ont, OntStatus } from "@/domain/entities/Ont";

function makeOnt(overrides: Partial<Ont>): Ont {
  return {
    id: "ont-1",
    oltId: "olt-1",
    oltName: "OLT1",
    portId: 1,
    ontId: 1,
    serialNumber: "RTEGC609D6CF",
    name: "Budi Santoso",
    description: "",
    status: OntStatus.ONLINE,
    lastSeenAt: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe("filterOntsByQuery", () => {
  const onts = [
    makeOnt({ id: "1", serialNumber: "RTEGC609D6CF", name: "Budi Santoso" }),
    makeOnt({ id: "2", serialNumber: "ZTEGC1234567", name: "Warnet Maju" }),
    makeOnt({ id: "3", serialNumber: "ABCDEF000001", name: "Budi Jaya" }),
  ];

  it("returns all ONTs when query is empty", () => {
    expect(filterOntsByQuery(onts, "")).toEqual(onts);
  });

  it("matches serial number case-insensitively", () => {
    expect(filterOntsByQuery(onts, "rtegc609")).toEqual([onts[0]]);
  });

  it("matches ONT name case-insensitively", () => {
    expect(filterOntsByQuery(onts, "warnet")).toEqual([onts[1]]);
  });

  it("matches any ONT sharing the name substring", () => {
    expect(filterOntsByQuery(onts, "budi")).toEqual([onts[0], onts[2]]);
  });

  it("returns empty array when nothing matches", () => {
    expect(filterOntsByQuery(onts, "zzz-not-found")).toEqual([]);
  });
});
