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

  // Showing everything is the row count itself.
  it("ends with the row count so everything can be shown", () => {
    const options = ontPageSizeOptions(200);

    expect(options[options.length - 1]).toBe(200);
  });

  // A table smaller than the largest fixed size already shows everything, so
  // appending the count would repeat the last option.
  it("does not repeat an option for a short table", () => {
    expect(ontPageSizeOptions(20)).toEqual([5, 10, 20]);
    expect(ontPageSizeOptions(3)).toEqual([5]);
  });

  it("never offers a page larger than the table", () => {
    for (const size of ontPageSizeOptions(37)) {
      expect(size).toBeLessThanOrEqual(37);
    }
  });
});
