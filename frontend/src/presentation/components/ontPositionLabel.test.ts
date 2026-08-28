import { describe, expect, it } from "vitest";
import { ontPositionLabel } from "./OntFilters";

// An ONU's address reads rack/card/pon, and every OLT here is a single shelf.
// The rack is shown rather than offered as a choice that cannot be made.
describe("ontPositionLabel", () => {
  it("reads as rack/card for a card", () => {
    expect(ontPositionLabel(3)).toBe("1/3");
  });

  it("reads as rack/card/pon for a port", () => {
    expect(ontPositionLabel(3, 1)).toBe("1/3/1");
  });
});
