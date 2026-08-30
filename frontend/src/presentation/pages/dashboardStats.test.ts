import { describe, expect, it } from "vitest";
import { OltStatus, OntStatus } from "@/domain/entities";
import type { Olt, Ont } from "@/domain/entities";
import {
  availabilityTone,
  formatAge,
  isPartialSummary,
  signalTone,
  summariseByOlt,
  summariseOlts,
  summariseOnts,
  uptimePercent,
  weakestSignals,
} from "./dashboardStats";

const olt = (status: OltStatus): Olt => ({ status }) as Olt;
const ont = (status: OntStatus): Ont => ({ status }) as Ont;

const namedOlt = (id: string, name: string): Olt =>
  ({ id, name, status: OltStatus.ONLINE }) as Olt;
const ontOf = (overrides: Partial<Ont>): Ont =>
  ({ status: OntStatus.ONLINE, ...overrides }) as Ont;

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

describe("summariseOnts", () => {
  it("groups the two fault states separately from offline", () => {
    // LOS and dying gasp are distinct faults an operator acts on differently,
    // so they must not be folded into a single offline count.
    const summary = summariseOnts([
      ont(OntStatus.ONLINE),
      ont(OntStatus.OFFLINE),
      ont(OntStatus.LOS),
      ont(OntStatus.DYING_GASP),
      ont(OntStatus.UNKNOWN),
    ]);

    expect(summary).toEqual({
      counted: 5,
      total: 5,
      online: 1,
      offline: 1,
      los: 1,
      dyingGasp: 1,
      unknown: 1,
    });
  });

  it("buckets every ONT it counted, leaving no remainder", () => {
    // An ONT in no bucket makes the parts stop adding up to the whole, which
    // reads as arithmetic the operator cannot trust.
    const summary = summariseOnts([
      ont(OntStatus.ONLINE),
      ont(OntStatus.UNKNOWN),
      ont(OntStatus.UNKNOWN),
    ]);

    const bucketed =
      summary.online +
      summary.offline +
      summary.los +
      summary.dyingGasp +
      summary.unknown;
    expect(bucketed).toBe(summary.counted);
  });

  it("keeps the server's total apart from the rows it received", () => {
    // The API caps a page at 500 rows. Counting rows would silently report a
    // 900-ONT network as 500.
    const summary = summariseOnts([ont(OntStatus.ONLINE)], 900);

    expect(summary.counted).toBe(1);
    expect(summary.total).toBe(900);
    expect(isPartialSummary(summary)).toBe(true);
  });

  it("is not partial when every row arrived", () => {
    expect(isPartialSummary(summariseOnts([ont(OntStatus.ONLINE)], 1))).toBe(
      false,
    );
  });
});

describe("summariseByOlt", () => {
  it("attributes each ONT to the OLT that owns it", () => {
    const rows = summariseByOlt(
      [namedOlt("o1", "Depok"), namedOlt("o2", "Bekasi")],
      [
        ontOf({ oltId: "o1" }),
        ontOf({ oltId: "o1", status: OntStatus.LOS }),
        ontOf({ oltId: "o2" }),
      ],
    );

    expect(rows.find((r) => r.oltId === "o1")).toMatchObject({
      ontTotal: 2,
      online: 1,
      impaired: 1,
      availability: 50,
    });
  });

  it("puts the OLT needing attention first", () => {
    const rows = summariseByOlt(
      [namedOlt("o1", "Healthy"), namedOlt("o2", "Struggling")],
      [
        ontOf({ oltId: "o1" }),
        ontOf({ oltId: "o2" }),
        ontOf({ oltId: "o2", status: OntStatus.OFFLINE }),
      ],
    );

    expect(rows.map((r) => r.oltName)).toEqual(["Struggling", "Healthy"]);
  });

  it("keeps an OLT with no ONTs but sorts it last rather than as an outage", () => {
    const rows = summariseByOlt(
      [namedOlt("o1", "Empty"), namedOlt("o2", "Busy")],
      [ontOf({ oltId: "o2", status: OntStatus.OFFLINE })],
    );

    expect(rows.map((r) => r.oltName)).toEqual(["Busy", "Empty"]);
    expect(rows[1].availability).toBeNull();
  });
});

describe("weakestSignals", () => {
  it("orders by the least light received", () => {
    const signals = weakestSignals([
      ontOf({ id: "1", name: "A", rxPower: -21.4 }),
      ontOf({ id: "2", name: "B", rxPower: -28.9 }),
      ontOf({ id: "3", name: "C", rxPower: -25.1 }),
    ]);

    expect(signals.map((s) => s.name)).toEqual(["B", "C", "A"]);
  });

  it("ignores ONTs that are not online", () => {
    // An offline ONT's last reading is the worst number in the table and would
    // crowd out the links that are still up and still worth saving.
    const signals = weakestSignals([
      ontOf({ id: "1", name: "Dark", status: OntStatus.LOS, rxPower: -40 }),
      ontOf({ id: "2", name: "Live", rxPower: -26 }),
    ]);

    expect(signals.map((s) => s.name)).toEqual(["Live"]);
  });

  it("ignores ONTs with no reading at all", () => {
    const signals = weakestSignals([
      ontOf({ id: "1", name: "NoReading", rxPower: null }),
      ontOf({ id: "2", name: "Reading", rxPower: -20 }),
    ]);

    expect(signals.map((s) => s.name)).toEqual(["Reading"]);
  });

  it("falls back to the serial when an ONT was never named", () => {
    const signals = weakestSignals([
      ontOf({ id: "1", name: "", serialNumber: "ZTEGC0FFEE01", rxPower: -20 }),
    ]);

    expect(signals[0].name).toBe("ZTEGC0FFEE01");
  });

  it("returns at most the requested number", () => {
    const many = Array.from({ length: 9 }, (_, i) =>
      ontOf({ id: `t${i}`, rxPower: -20 - i }),
    );

    expect(weakestSignals(many)).toHaveLength(5);
    expect(weakestSignals(many, 2)).toHaveLength(2);
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
