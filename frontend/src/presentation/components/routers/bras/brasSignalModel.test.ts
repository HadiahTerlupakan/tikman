import { describe, expect, it } from "vitest";
import type {
  BrasRadius,
  BrasSnapshot,
  RouterBrasView,
} from "@/domain/entities";
import { brasSignals, sinceText } from "./brasSignalModel";

const AT = "2026-11-20T10:00:00Z";
const NOW = new Date("2026-11-20T10:01:00Z");

function server(over: Partial<BrasRadius> = {}): BrasRadius {
  return {
    id: "*1",
    address: "192.0.2.10",
    requests: 100,
    accepts: 90,
    rejects: 10,
    timeouts: 0,
    pending: 0,
    rttMs: 4,
    answering: true,
    ...over,
  };
}

function snapshot(over: Partial<BrasSnapshot> = {}): BrasSnapshot {
  return {
    sessions: 1842,
    cores: [
      { cpu: "cpu0", load: 71 },
      { cpu: "cpu1", load: 20 },
      { cpu: "cpu2", load: 30 },
      { cpu: "cpu3", load: 31 },
    ],
    pools: [{ name: "pool-pppoe", used: 1000, total: 2000, free: 1000 }],
    radius: [server()],
    topUsers: [],
    queuesFound: true,
    ...over,
  };
}

function view(over: Partial<RouterBrasView> = {}): RouterBrasView {
  return {
    bras: snapshot(),
    brasAt: AT,
    samples: [],
    sessions15mAgo: 1845,
    radiusAccepts1h: 200,
    radiusRejects1h: 12,
    radiusTimeouts1h: 0,
    drops: [],
    ...over,
  };
}

describe("brasSignals: sessions", () => {
  it("shows the count now and fifteen minutes ago", () => {
    const { sessions } = brasSignals(view(), NOW);
    expect(sessions).toEqual({
      level: "ok",
      value: "1.842",
      context: "15 menit lalu 1.845",
    });
  });

  it("is red when a drop falls within 15 minutes of the reading", () => {
    const drops = [{ time: "2026-11-20T09:50:00Z", lost: 312 }];
    expect(brasSignals(view({ drops }), NOW).sessions.level).toBe("bad");
  });

  it("forgets a drop older than 15 minutes", () => {
    const drops = [{ time: "2026-11-20T09:40:00Z", lost: 312 }];
    expect(brasSignals(view({ drops }), NOW).sessions.level).toBe("ok");
  });

  it("shows a missing earlier sample as a dash, never 0", () => {
    const { sessions } = brasSignals(view({ sessions15mAgo: null }), NOW);
    expect(sessions.context).toBe("15 menit lalu —");
  });
});

describe("brasSignals: RADIUS", () => {
  const radius = (over: Partial<RouterBrasView>, servers?: BrasRadius[]) =>
    brasSignals(
      view({ bras: snapshot(servers ? { radius: servers } : {}), ...over }),
      NOW,
    ).radius;

  it("is grey and unused when the BRAS has no RADIUS entry, never red", () => {
    expect(radius({}, [])).toMatchObject({
      level: "none",
      value: "Tidak dipakai",
    });
  });

  it("is green while every server answers, counting rejects in the hour", () => {
    expect(radius({})).toEqual({
      level: "ok",
      value: "Menjawab",
      context: "12 login ditolak dalam 1 jam",
    });
  });

  it("is red when no server answers", () => {
    const servers = [
      server({ answering: false }),
      server({ answering: false }),
    ];
    expect(radius({}, servers).level).toBe("bad");
  });

  it("is amber when only some servers answer", () => {
    const servers = [server(), server({ answering: false })];
    expect(radius({}, servers)).toMatchObject({
      level: "warn",
      value: "1 dari 2 menjawab",
    });
  });

  it("is amber when there were timeouts in the hour", () => {
    expect(radius({ radiusTimeouts1h: 1 }).level).toBe("warn");
  });

  it("is amber when rejects exceed 10% of accepts in the hour", () => {
    expect(radius({ radiusRejects1h: 21 }).level).toBe("warn");
    expect(radius({ radiusRejects1h: 20 }).level).toBe("ok");
  });

  it("shows unknown rejects as a dash", () => {
    expect(radius({ radiusRejects1h: null }).context).toBe(
      "— login ditolak dalam 1 jam",
    );
  });
});

describe("brasSignals: CPU", () => {
  const cpu = (load: number) =>
    brasSignals(
      view({ bras: snapshot({ cores: [{ cpu: "cpu0", load }] }) }),
      NOW,
    ).cpu.level;

  it("shows the busiest core and the average", () => {
    expect(brasSignals(view(), NOW).cpu).toEqual({
      level: "warn",
      value: "71%",
      context: "rata-rata 4 core 38%",
    });
  });

  it("is amber from 70% and red from 90%", () => {
    expect(cpu(69)).toBe("ok");
    expect(cpu(70)).toBe("warn");
    expect(cpu(89)).toBe("warn");
    expect(cpu(90)).toBe("bad");
  });
});

describe("brasSignals: pool", () => {
  const pool = (pools: BrasSnapshot["pools"]) =>
    brasSignals(view({ bras: snapshot({ pools }) }), NOW).pool;

  it("shows the fewest free addresses and the fullest pool", () => {
    expect(
      pool([
        { name: "pool-pppoe", used: 1842, total: 2032, free: 190 },
        { name: "pool-isolir", used: 23, total: 254, free: 231 },
      ]),
    ).toEqual({
      level: "warn",
      value: "190",
      context: "pool-pppoe terpakai 91%",
    });
  });

  it("is red below 2% free", () => {
    expect(
      pool([{ name: "pool-a", used: 990, total: 1000, free: 10 }]).level,
    ).toBe("bad");
  });

  it("leaves out a pool whose total is unknown", () => {
    expect(
      pool([
        { name: "pool-v6", used: 5, total: null, free: null },
        { name: "pool-a", used: 10, total: 100, free: 90 },
      ]),
    ).toMatchObject({ level: "ok", value: "90" });
  });

  it("is a grey dash when no pool's total is known", () => {
    expect(
      pool([{ name: "pool-v6", used: 5, total: null, free: null }]),
    ).toMatchObject({ level: "none", value: "—" });
  });
});

describe("brasSignals: staleness", () => {
  it("greys every signal out when the reading is older than 5 minutes", () => {
    const signals = brasSignals(
      view({ bras: snapshot({ radius: [server({ answering: false })] }) }),
      new Date("2026-11-20T10:06:00Z"),
    );
    expect(Object.values(signals).map((s) => s.level)).toEqual([
      "none",
      "none",
      "none",
      "none",
    ]);
    expect(signals.sessions.value).toBe("1.842");
  });

  it("is all dashes before the BRAS was ever read", () => {
    const signals = brasSignals(view({ bras: null, brasAt: null }), NOW);
    expect(signals.cpu).toMatchObject({ level: "none", value: "—" });
  });

  it("says how long ago the last reading was", () => {
    expect(sinceText(AT, new Date("2026-11-20T10:07:30Z"))).toBe(
      "7 menit lalu",
    );
    expect(sinceText(AT, new Date("2026-11-20T13:00:00Z"))).toBe("3 jam lalu");
  });
});
