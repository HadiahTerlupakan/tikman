import { describe, expect, it } from "vitest";
import { matchesOntFilters } from "./ontFilters";
import { OntStatus, type Ont } from "@/domain/entities";

function ont(overrides: Partial<Ont> = {}): Ont {
  return {
    id: "ont-1",
    oltId: "olt-1",
    slot: 3,
    portId: 1,
    ontId: 15,
    serialNumber: "HWTCB403E8A0",
    name: "",
    description: "",
    status: "online",
    ...overrides,
  } as Ont;
}

describe("matchesOntFilters", () => {
  // The card was collected and then ignored, so picking one changed nothing.
  it("keeps only the selected card", () => {
    expect(matchesOntFilters(ont({ slot: 3 }), { slot: 3 })).toBe(true);
    expect(matchesOntFilters(ont({ slot: 4 }), { slot: 3 })).toBe(false);
  });

  // Without the card, port 1 matched port 1 on every card at once.
  it("does not mix the same port across cards", () => {
    const onCardFour = ont({ slot: 4, portId: 1 });

    expect(matchesOntFilters(onCardFour, { portId: 1 })).toBe(true);
    expect(matchesOntFilters(onCardFour, { slot: 3, portId: 1 })).toBe(false);
  });

  // An ONT the poll has not placed on a card cannot answer the question.
  it("leaves out an ONT with no card when one is selected", () => {
    expect(matchesOntFilters(ont({ slot: undefined }), { slot: 3 })).toBe(
      false,
    );
    expect(matchesOntFilters(ont({ slot: undefined }), {})).toBe(true);
  });

  it("matches a serial fragment regardless of case", () => {
    expect(matchesOntFilters(ont(), { searchText: "b403e8a0" })).toBe(true);
    expect(matchesOntFilters(ont(), { searchText: "RTEG" })).toBe(false);
  });

  it("applies the OLT and status filters", () => {
    expect(matchesOntFilters(ont(), { oltId: "olt-2" })).toBe(false);
    expect(
      matchesOntFilters(ont({ status: OntStatus.LOS }), {
        status: OntStatus.ONLINE,
      }),
    ).toBe(false);
  });
});
