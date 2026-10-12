import { describe, expect, it } from "vitest";
import { savableFingerprint } from "./routerTestSteps";

const FP = "AA:BB:CC";

describe("savableFingerprint over TLS", () => {
  it("is null before the fingerprint has been confirmed", () => {
    expect(savableFingerprint("rest", { fingerprint: FP }, null)).toBeNull();
    expect(savableFingerprint("rest", null, null)).toBeNull();
  });

  it("returns the step-1 fingerprint once step 2 reached the API and SNMP", () => {
    expect(
      savableFingerprint(
        "api-ssl",
        { fingerprint: FP },
        { fingerprint: FP, boardName: "CCR2004", version: "7.15" },
      ),
    ).toBe(FP);
  });

  it("stays null when step 2 reports an API error", () => {
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP },
        { fingerprint: FP, apiError: "401 unauthorized" },
      ),
    ).toBeNull();
  });

  it("stays null when SNMP failed in either step", () => {
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP, snmpError: "timeout" },
        { fingerprint: FP, boardName: "CCR2004" },
      ),
    ).toBeNull();
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP },
        { fingerprint: FP, snmpError: "timeout", boardName: "CCR2004" },
      ),
    ).toBeNull();
  });
});

describe("savableFingerprint over plain API", () => {
  it("is savable with no fingerprint after one passing step", () => {
    expect(
      savableFingerprint(
        "api",
        { boardName: "CCR2004", version: "7.24" },
        null,
      ),
    ).toBe("");
  });

  it("stays null when the single step failed", () => {
    expect(
      savableFingerprint("api", { apiError: "router rejected" }, null),
    ).toBeNull();
    expect(
      savableFingerprint("api", { snmpError: "timeout", boardName: "x" }, null),
    ).toBeNull();
    expect(savableFingerprint("api", null, null)).toBeNull();
  });
});
