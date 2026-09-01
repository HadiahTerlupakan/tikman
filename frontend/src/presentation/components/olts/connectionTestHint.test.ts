import { describe, expect, it } from "vitest";
import { connectionTestHint } from "./connectionTestHint";

describe("connectionTestHint", () => {
  it("says the device was already reached when only SNMP failed", () => {
    const hint = connectionTestHint("SNMP", ["Ping", "Telnet"]);

    // The probe's own message can only guess at "device unreachable", because
    // the SNMP client knows nothing of the two probes that just succeeded.
    expect(hint).toContain("Ping and Telnet");
    expect(hint).toContain("snmp-server community");
  });

  it("stays silent when SNMP failed without the device answering anything", () => {
    // Nothing reached the device, so unreachable is a live possibility and
    // ruling it out here would send an operator past the real fault.
    expect(connectionTestHint("SNMP", [])).toBeNull();
  });

  it("stays silent for a failure that is not SNMP", () => {
    expect(connectionTestHint("Telnet", ["Ping"])).toBeNull();
  });
});
