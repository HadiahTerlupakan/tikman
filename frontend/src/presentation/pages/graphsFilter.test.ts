import { describe, expect, it } from "vitest";
import { describeOntCoverage, filterOntsByQuery } from "./graphsFilter";
import type { Ont } from "@/domain/entities/Ont";

function ont(serialNumber: string, name = ""): Ont {
  return { id: serialNumber, serialNumber, name } as Ont;
}

describe("filterOntsByQuery", () => {
  it("matches a serial fragment regardless of case", () => {
    const onts = [ont("HWTCB403E8A0"), ont("RTEGC609DA61")];

    expect(filterOntsByQuery(onts, "b403e8a0")).toEqual([onts[0]]);
  });

  it("returns everything for an empty query", () => {
    const onts = [ont("HWTCB403E8A0")];

    expect(filterOntsByQuery(onts, "   ")).toEqual(onts);
  });
});

describe("describeOntCoverage", () => {
  it("reports the page and the match count when everything is loaded", () => {
    expect(describeOntCoverage(9, 200, 200, 200)).toBe("Showing 9 of 200 ONTs");
  });

  // The search box filters what was loaded, so an OLT past the endpoint's
  // ceiling has ONTs it silently cannot find. Reporting the loaded count as the
  // total hid that.
  it("says so when the OLT has more ONTs than were loaded", () => {
    expect(describeOntCoverage(9, 500, 500, 812)).toBe(
      "Showing 9 of 500 ONTs — only the first 500 of 812 are loaded, so narrow the filter to reach the rest",
    );
  });
});
