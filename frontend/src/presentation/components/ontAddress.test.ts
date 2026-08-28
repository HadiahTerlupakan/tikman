import { describe, expect, it } from "vitest";
import { ontAddressLabel, ontPositionLabel } from "./ontAddress";

// An ONU's address reads rack/card/pon, and every OLT here is a single shelf.
// The rack is shown rather than offered as a choice that cannot be made.
describe("ontPositionLabel", () => {
  it("reads as rack/card, and rack/card/pon with a port", () => {
    expect(ontPositionLabel(3)).toBe("1/3");
    expect(ontPositionLabel(3, 1)).toBe("1/3/1");
  });
});

describe("ontAddressLabel", () => {
  // The CLI address. Showing PON and ONU alone left the card out, so an ONU on
  // card 3 and one on card 4 read the same.
  it("matches the address the OLT uses", () => {
    expect(ontAddressLabel(3, 1, 18)).toBe("1/3/1:18");
    expect(ontAddressLabel(4, 16, 2)).toBe("1/4/16:2");
  });

  // An ONT the poll has not placed on a card yet cannot claim an address.
  it("does not invent a card it does not know", () => {
    expect(ontAddressLabel(undefined, 1, 18)).toBe("PON 1 · ONU 18");
  });
});
