import { describe, expect, it } from "vitest";
import { ontPageSizeOptions } from "./ontPageSize";

describe("ontPageSizeOptions", () => {
  // The old list ended in the string "all", which Ant Design passes back as the
  // page size. It is not a number, so choosing it asked for NaN rows.
  it("offers only numbers", () => {
    for (const size of ontPageSizeOptions(200)) {
      expect(Number.isInteger(size)).toBe(true);
    }
  });

  // The row count used to be offered as a page size, which worked while the
  // browser already held every row. The database pages the list now, so that
  // option would be a request for the whole network.
  it("never offers to show the whole network at once", () => {
    expect(ontPageSizeOptions(200)).toEqual([5, 10, 20, 50, 100]);
    expect(ontPageSizeOptions(300000)).toEqual([5, 10, 20, 50, 100]);
  });

  it("offers only pages a result this size can fill", () => {
    expect(ontPageSizeOptions(20)).toEqual([5, 10]);
    expect(ontPageSizeOptions(3)).toEqual([5]);
  });

  it("never offers a page larger than the table", () => {
    for (const size of ontPageSizeOptions(37)) {
      expect(size).toBeLessThanOrEqual(37);
    }
  });
});
