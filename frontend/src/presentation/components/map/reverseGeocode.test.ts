import { describe, expect, it } from "vitest";
import { addressFromGeocode } from "./reverseGeocode";

describe("addressFromGeocode", () => {
  it("takes the most specific result Google returned", () => {
    // Google orders results most specific first: the street, then the village,
    // then the province. The first is the one someone can drive to.
    expect(
      addressFromGeocode([
        { formatted_address: "Jl. Raya Cariu No. 12, Bogor" },
        { formatted_address: "Kabupaten Bogor, Jawa Barat" },
      ]),
    ).toBe("Jl. Raya Cariu No. 12, Bogor");
  });

  it("leaves the field empty when Google found nothing", () => {
    // Better blank and typed than confidently wrong: someone drives to this.
    expect(addressFromGeocode([])).toBe("");
    expect(addressFromGeocode(null)).toBe("");
    expect(addressFromGeocode(undefined)).toBe("");
  });

  it("skips a result carrying no address at all", () => {
    expect(
      addressFromGeocode([{}, { formatted_address: "Jl. Raya Cariu" }]),
    ).toBe("Jl. Raya Cariu");
  });

  it("ignores an address that is only whitespace", () => {
    expect(addressFromGeocode([{ formatted_address: "   " }])).toBe("");
  });
});
