import { describe, expect, it } from "vitest";
import {
  buildOdpDto,
  odcFeeds,
  odpFormProblem,
  type OdpFormValues,
} from "./plantForm";

const here = { latitude: -6.4, longitude: 106.8 };

const values = (over: Partial<OdpFormValues> = {}): OdpFormValues => ({
  code: "ODP-01",
  portCount: 8,
  parentKind: "odc",
  odcId: "odc-1",
  ...over,
});

describe("buildOdpDto", () => {
  it("sends only the cabinet when the box hangs off one", () => {
    const dto = buildOdpDto(
      values({
        parentKind: "odc",
        odcId: "odc-1",
        oltId: "olt-1",
        slot: 1,
        portId: 2,
      }),
      here,
    );

    // The form keeps both halves as the operator switches between them; sending
    // both would be refused by the server for naming two parents.
    expect(dto.odcId).toBe("odc-1");
    expect(dto.oltId).toBeUndefined();
    expect(dto.slot).toBeUndefined();
    expect(dto.portId).toBeUndefined();
  });

  it("sends only the PON port when the box hangs off one", () => {
    const dto = buildOdpDto(
      values({
        parentKind: "pon",
        odcId: "odc-1",
        oltId: "olt-1",
        slot: 1,
        portId: 2,
      }),
      here,
    );

    expect(dto.odcId).toBeUndefined();
    expect(dto.oltId).toBe("olt-1");
    expect(dto.slot).toBe(1);
    expect(dto.portId).toBe(2);
  });

  it("drops blank optional text rather than storing empty strings", () => {
    const dto = buildOdpDto(values({ notes: "" }), here);

    expect(dto.notes).toBeUndefined();
  });

  it("carries the coordinates the map was clicked at", () => {
    const dto = buildOdpDto(values(), here);

    expect(dto.latitude).toBe(-6.4);
    expect(dto.longitude).toBe(106.8);
  });
});

describe("odpFormProblem", () => {
  it("asks for a location before anything else", () => {
    expect(odpFormProblem(values(), undefined)).toMatch(/klik di peta/i);
  });

  it("asks for the cabinet when that is the parent", () => {
    expect(odpFormProblem(values({ odcId: undefined }), here)).toMatch(/ODC/);
  });

  it("asks for the whole PON address, not just the OLT", () => {
    // A PON parent without slot and port is not an address, and the server
    // refuses it — better said here than after a round trip.
    expect(
      odpFormProblem(values({ parentKind: "pon", oltId: "olt-1" }), here),
    ).toMatch(/slot dan port/i);
  });

  it("is silent when the box can be saved", () => {
    expect(odpFormProblem(values(), here)).toBeNull();
    expect(
      odpFormProblem(
        values({ parentKind: "pon", oltId: "olt-1", slot: 1, portId: 2 }),
        here,
      ),
    ).toBeNull();
  });
});

describe("odcFeeds", () => {
  const full = { oltId: "olt-1", slot: 1, portId: 4, splitterOutputs: 8 };

  it("sends the feed when the whole PON address is named", () => {
    expect(odcFeeds(full)).toEqual([full]);
  });

  it("sends nothing when the feeder is not spliced yet", () => {
    // Recording where a cabinet stands before its feeder exists is ordinary
    // field order, not an incomplete form.
    expect(odcFeeds({})).toBeUndefined();
  });

  it("sends nothing for half an address", () => {
    // The server would refuse it, and refusing it would take the whole cabinet
    // down with it, since the two are saved together.
    expect(odcFeeds({ ...full, portId: undefined })).toBeUndefined();
    expect(odcFeeds({ ...full, slot: undefined })).toBeUndefined();
    expect(odcFeeds({ ...full, oltId: undefined })).toBeUndefined();
  });

  it("sends nothing without a splitter ratio", () => {
    expect(odcFeeds({ ...full, splitterOutputs: undefined })).toBeUndefined();
  });

  it("keeps a card or port numbered zero, which chassis do use", () => {
    expect(odcFeeds({ ...full, slot: 0, portId: 0 })).toEqual([
      { ...full, slot: 0, portId: 0 },
    ]);
  });
});
