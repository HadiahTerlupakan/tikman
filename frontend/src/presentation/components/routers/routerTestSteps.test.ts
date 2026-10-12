import { describe, expect, it } from "vitest";
import { confirmedFingerprint } from "./routerTestSteps";

const FP = "AA:BB:CC";

describe("confirmedFingerprint", () => {
  it("is null before the fingerprint has been confirmed", () => {
    expect(confirmedFingerprint({ fingerprint: FP }, null)).toBeNull();
    expect(confirmedFingerprint(null, null)).toBeNull();
  });

  it("returns the step-1 fingerprint once step 2 reached REST and SNMP", () => {
    expect(
      confirmedFingerprint(
        { fingerprint: FP },
        { fingerprint: FP, boardName: "CCR2004", version: "7.15" },
      ),
    ).toBe(FP);
  });

  it("stays null when step 2 reports a REST error", () => {
    expect(
      confirmedFingerprint(
        { fingerprint: FP },
        { fingerprint: FP, apiError: "401 unauthorized" },
      ),
    ).toBeNull();
  });

  it("stays null when SNMP failed in either step", () => {
    expect(
      confirmedFingerprint(
        { fingerprint: FP, snmpError: "timeout" },
        { fingerprint: FP, boardName: "CCR2004" },
      ),
    ).toBeNull();
    expect(
      confirmedFingerprint(
        { fingerprint: FP },
        { fingerprint: FP, snmpError: "timeout", boardName: "CCR2004" },
      ),
    ).toBeNull();
  });

  it("stays null when step 1 never saw a certificate", () => {
    expect(
      confirmedFingerprint({}, { boardName: "CCR2004", version: "7.15" }),
    ).toBeNull();
  });
});
