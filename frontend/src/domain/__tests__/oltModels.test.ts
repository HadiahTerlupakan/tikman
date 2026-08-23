import { describe, it, expect } from "vitest";
import { OltModel, OLT_MODELS } from "../entities/Olt";

// The backend validates `model` with `oneof=zte_c300 zte_c320 hsgq` in
// CreateOLTRequest/UpdateOLTRequest. If this list drifts from that one, the
// create form posts a value the API rejects with a 400 and the table falls back
// to rendering the raw string, so the contract is worth pinning.
const backendAcceptedModels = ["zte_c300", "zte_c320", "hsgq"];

describe("OLT model list", () => {
  it("offers exactly the models the backend accepts", () => {
    expect(OLT_MODELS.map((m) => m.value)).toEqual(backendAcceptedModels);
  });

  it("has a label for every model, so none renders as a raw value", () => {
    for (const model of Object.values(OltModel)) {
      const entry = OLT_MODELS.find((m) => m.value === model);
      expect(entry?.label).toBeTruthy();
    }
  });

  it("warns that the HSGQ OIDs are unverified", () => {
    const hsgq = OLT_MODELS.find((m) => m.value === OltModel.HSGQ);
    expect(hsgq?.hint).toBe("OID belum diverifikasi");
  });
});
