import { describe, expect, it } from "vitest";
import { OltStatus, OntStatus } from "@/domain/entities";
import type { Olt, Ont } from "@/domain/entities";
import {
  availabilityTone,
  summariseOlts,
  summariseOnts,
  uptimePercent,
} from "./dashboardStats";

const olt = (status: OltStatus): Olt => ({ status }) as Olt;
const ont = (status: OntStatus): Ont => ({ status }) as Ont;

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
      total: 5,
      online: 1,
      offline: 1,
      los: 1,
      dyingGasp: 1,
    });
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
