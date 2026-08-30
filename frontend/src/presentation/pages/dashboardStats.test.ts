import { describe, expect, it } from "vitest";
import { OltStatus } from "@/domain/entities";
import type {
  Olt,
  OltBreakdownRow,
  WeakSignalReading,
} from "@/domain/entities";
import {
  availabilityTone,
  formatAge,
  rankOltRows,
  signalTone,
  summariseOlts,
  toWeakSignals,
  uptimePercent,
} from "./dashboardStats";

const olt = (status: OltStatus): Olt => ({ status }) as Olt;

// The server counts the ONTs; these are the rows it sends back.
const breakdown = (
  name: string,
  ontTotal: number,
  online: number,
): OltBreakdownRow => ({
  oltId: name,
  oltName: name,
  oltStatus: OltStatus.ONLINE,
  ontTotal,
  online,
  impaired: ontTotal - online,
});

const reading = (overrides: Partial<WeakSignalReading>): WeakSignalReading => ({
  id: "t1",
  name: "ONT",
  serialNumber: "ZTEGC0FFEE01",
  oltName: "Cariu",
  rxPower: -20,
  ...overrides,
});

describe("summariseOlts", () => {
  it("counts each status independently", () => {
    const summary = summariseOlts([
      olt(OltStatus.ONLINE),
      olt(OltStatus.ONLINE),
      olt(OltStatus.OFFLINE),
      olt(OltStatus.ERROR),
    ]);

    expect(summary).toEqual({ total: 4, online: 2, offline: 1, error: 1 });
  });

  it("returns zeroes for no data rather than throwing", () => {
    expect(summariseOlts(undefined)).toEqual({
      total: 0,
      online: 0,
      offline: 0,
      error: 0,
    });
  });
});

describe("rankOltRows", () => {
  it("derives availability from the counts the server sent", () => {
    const rows = rankOltRows([breakdown("Depok", 2, 1)]);

    expect(rows[0]).toMatchObject({ ontTotal: 2, online: 1, availability: 50 });
  });

  it("puts the OLT needing attention first", () => {
    const rows = rankOltRows([
      breakdown("Healthy", 1, 1),
      breakdown("Struggling", 2, 1),
    ]);

    expect(rows.map((r) => r.oltName)).toEqual(["Struggling", "Healthy"]);
  });

  it("keeps an OLT with no ONTs but sorts it last rather than as an outage", () => {
    // Availability of null is "nothing to measure". Sorting it as 0% would put
    // an idle OLT above one that is actually losing subscribers.
    const rows = rankOltRows([
      breakdown("Empty", 0, 0),
      breakdown("Busy", 1, 0),
    ]);

    expect(rows.map((r) => r.oltName)).toEqual(["Busy", "Empty"]);
    expect(rows[1].availability).toBeNull();
  });

  it("returns an empty list rather than throwing before the first response", () => {
    expect(rankOltRows(undefined)).toEqual([]);
  });
});

describe("toWeakSignals", () => {
  it("falls back to the serial when an ONT was never named", () => {
    // The OLT labels an ONU inconsistently, and a blank row is one a technician
    // cannot match to a box in the field.
    const signals = toWeakSignals([
      reading({ name: "", serialNumber: "ZTEGC0FFEE01" }),
    ]);

    expect(signals[0].name).toBe("ZTEGC0FFEE01");
  });

  it("keeps a name the OLT did give", () => {
    expect(toWeakSignals([reading({ name: "Heru Kurniawan" })])[0].name).toBe(
      "Heru Kurniawan",
    );
  });

  it("returns an empty list rather than throwing before the first response", () => {
    expect(toWeakSignals(undefined)).toEqual([]);
  });
});

describe("signalTone", () => {
  it("calls a link below Class B+ sensitivity a fault, not a warning", () => {
    expect(signalTone(-27.5)).toBe("danger");
    expect(signalTone(-25)).toBe("warning");
    expect(signalTone(-19)).toBe("success");
  });
});

describe("availabilityTone", () => {
  it("is healthy at high availability", () => {
    expect(availabilityTone(97)).toBe("success");
  });

  it("warns once availability slips below the threshold", () => {
    expect(availabilityTone(94)).toBe("warning");
  });

  it("is critical when most of the network is down", () => {
    expect(availabilityTone(58)).toBe("danger");
  });

  it("is neutral when there is nothing to measure", () => {
    // A green ring over "—" would claim health the data does not support.
    expect(availabilityTone(null)).toBe("neutral");
  });
});

describe("uptimePercent", () => {
  it("rounds to a whole percent", () => {
    expect(uptimePercent(2, 3)).toBe(67);
  });

  it("is null when there is nothing to measure, so callers can show a dash", () => {
    // Returning 0 here would render "0% uptime" for an empty system, which reads
    // as an outage rather than as no data.
    expect(uptimePercent(0, 0)).toBeNull();
  });

  it("is 100 when every unit is online", () => {
    expect(uptimePercent(4, 4)).toBe(100);
  });
});

describe("formatAge", () => {
  it("keeps seconds so a fresh poll does not read as stale", () => {
    expect(formatAge(3_000)).toBe("3s ago");
    expect(formatAge(90_000)).toBe("1m ago");
    expect(formatAge(7_200_000)).toBe("2h ago");
  });

  it("never renders a negative age from a clock that ran ahead", () => {
    expect(formatAge(-500)).toBe("0s ago");
  });
});
