import { describe, expect, it } from "vitest";
import { cardOptions, portOptions, type TopologyCard } from "./ponOptions";

const topology: TopologyCard[] = [
  { slot: 9, ports: [{ portId: 2 }, { portId: 1 }] },
  { slot: 3, ports: [{ portId: 5 }] },
  { slot: 12, ports: [] },
];

describe("cardOptions", () => {
  it("offers the cards the chassis reported, in slot order", () => {
    expect(cardOptions(topology).map((option) => option.value)).toEqual([
      3, 9, 12,
    ]);
  });

  it("names them the way the chassis does", () => {
    expect(cardOptions(topology)[0].label).toBe("Card 3");
  });

  it("offers nothing for an OLT that has never been discovered", () => {
    // Better an empty list the form can explain than a free number field that
    // records a box hanging off a card the chassis does not have.
    expect(cardOptions(undefined)).toEqual([]);
    expect(cardOptions([])).toEqual([]);
  });
});

describe("portOptions", () => {
  it("offers only the ports on the card that was picked", () => {
    expect(portOptions(topology, 9).map((option) => option.value)).toEqual([
      1, 2,
    ]);
  });

  it("offers nothing until a card is picked", () => {
    expect(portOptions(topology, undefined)).toEqual([]);
  });

  it("offers nothing for a card the chassis does not have", () => {
    // This is what makes changing the card empty the port list rather than
    // leave a port number from the previous card standing.
    expect(portOptions(topology, 7)).toEqual([]);
  });

  it("offers nothing for a card reported with no ports", () => {
    expect(portOptions(topology, 12)).toEqual([]);
  });
});
