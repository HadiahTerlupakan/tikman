import { describe, expect, it } from "vitest";
import { freePorts } from "./odpPorts";

describe("freePorts", () => {
  it("offers every port on an empty box", () => {
    expect(freePorts(4, [])).toEqual([1, 2, 3, 4]);
  });

  it("leaves out the ports another subscriber holds", () => {
    expect(freePorts(4, [2, 3])).toEqual([1, 4]);
  });

  it("keeps the subscriber's own port on the list", () => {
    // Re-patching to the port already held is not a conflict, and dropping it
    // would make the current assignment unselectable in its own form.
    expect(freePorts(4, [2, 3], 3)).toEqual([1, 3, 4]);
  });

  it("offers nothing on a full box", () => {
    expect(freePorts(2, [1, 2])).toEqual([]);
  });
});
