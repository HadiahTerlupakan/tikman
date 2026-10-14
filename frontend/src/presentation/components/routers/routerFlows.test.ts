import { describe, expect, it } from "vitest";
import type {
  FlowDestinations,
  FlowPeering,
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import {
  asnLabel,
  asnTypeLabel,
  countryFlag,
  countryName,
  flowChartRows,
  flowStale,
  peakWindowLabel,
  peeringChartRows,
  peeringLabels,
  ownPrefixesAvailable,
  popLabel,
  topIpsAvailable,
} from "./routerFlows";

const NOW = new Date("2026-10-12T12:00:00Z");
const DAY_HOURS = 24;

describe("countryFlag", () => {
  it("turns an ISO code into its flag and anything else into nothing", () => {
    expect(countryFlag("ID")).toBe("\u{1F1EE}\u{1F1E9}");
    expect(countryFlag("JP")).toBe("\u{1F1EF}\u{1F1F5}");
    expect(countryFlag("Unknown")).toBe("");
    expect(countryFlag("")).toBe("");
    expect(countryFlag("us")).toBe("");
  });
});

describe("asnTypeLabel", () => {
  it("names PeeringDB's types in Indonesian and passes an unknown one through", () => {
    expect(asnTypeLabel("Content")).toBe("Konten");
    expect(asnTypeLabel("Cable/DSL/ISP")).toBe("ISP");
    expect(asnTypeLabel("Satellite")).toBe("Satellite");
    expect(asnTypeLabel("")).toBe("—");
  });
});

describe("countryName", () => {
  it("names the country in Indonesian", () => {
    expect(countryName("JP")).toBe("Jepang");
  });
});

describe("asnLabel", () => {
  it("names ASN 0 Lainnya and shows the number beside a name", () => {
    expect(asnLabel(0, "Lainnya")).toBe("Lainnya");
    expect(asnLabel(64496, "EXAMPLE-NET-A")).toBe("EXAMPLE-NET-A (AS64496)");
    expect(asnLabel(64497, "")).toBe("AS64497");
  });
});

describe("popLabel", () => {
  it("names the POP by its interface, and null as outside every POP", () => {
    const labels = new Map([["pop-a", "POP A"]]);
    expect(popLabel("pop-a", labels)).toBe("POP A");
    expect(popLabel(null, labels)).toBe("Lainnya (di luar blok POP)");
  });
});

describe("flowChartRows", () => {
  const data: FlowDestinations = {
    stepSeconds: 60,
    series: [
      {
        asn: 64496,
        name: "EXAMPLE-NET-A",
        points: [
          { time: "2026-10-12T03:01:00Z", bps: 200 },
          { time: "2026-10-12T03:00:00Z", bps: 100 },
        ],
      },
      { asn: 64497, name: "", points: [] },
      {
        asn: 0,
        name: "Lainnya",
        points: [{ time: "2026-10-12T03:01:00Z", bps: 5 }],
      },
    ],
  };

  it("pivots series into time-ordered rows, zero where a series has no point", () => {
    const { rows, series } = flowChartRows(data);

    expect(series).toEqual(["EXAMPLE-NET-A (AS64496)", "Lainnya"]);
    expect(rows).toEqual([
      {
        time: "2026-10-12T03:00:00Z",
        "EXAMPLE-NET-A (AS64496)": 100,
        Lainnya: 0,
      },
      {
        time: "2026-10-12T03:01:00Z",
        "EXAMPLE-NET-A (AS64496)": 200,
        Lainnya: 5,
      },
    ]);
  });

  it("aligns by timestamp, not index, when series have partly different times", () => {
    const { rows } = flowChartRows({
      stepSeconds: 60,
      series: [
        {
          asn: 64496,
          name: "A",
          points: [
            { time: "2026-10-12T03:00:00Z", bps: 1 },
            { time: "2026-10-12T03:01:00Z", bps: 2 },
          ],
        },
        {
          asn: 64497,
          name: "B",
          points: [
            { time: "2026-10-12T03:01:00Z", bps: 20 },
            { time: "2026-10-12T03:02:00Z", bps: 30 },
          ],
        },
      ],
    });

    expect(rows).toEqual([
      { time: "2026-10-12T03:00:00Z", "A (AS64496)": 1, "B (AS64497)": 0 },
      { time: "2026-10-12T03:01:00Z", "A (AS64496)": 2, "B (AS64497)": 20 },
      { time: "2026-10-12T03:02:00Z", "A (AS64496)": 0, "B (AS64497)": 30 },
    ]);
  });

  it("draws nothing before data arrives", () => {
    expect(flowChartRows(undefined)).toEqual({ rows: [], series: [] });
  });
});

describe("topIpsAvailable", () => {
  it("allows the 7-day preset and nothing reaching further back", () => {
    expect(
      topIpsAvailable({ kind: "relative", hours: 7 * DAY_HOURS }, NOW),
    ).toBe(true);
    expect(
      topIpsAvailable({ kind: "relative", hours: 30 * DAY_HOURS }, NOW),
    ).toBe(false);
    expect(
      topIpsAvailable(
        {
          kind: "absolute",
          from: "2026-10-04T12:00:00Z",
          to: "2026-10-05T12:00:00Z",
        },
        NOW,
      ),
    ).toBe(false);
  });
});

describe("flowStale", () => {
  const router = (over: Partial<MikrotikRouter>) =>
    ({
      flowEnabled: true,
      flowLastSeenAt: null,
      ...over,
    }) as MikrotikRouter;
  const minutesAgo = (m: number) =>
    new Date(NOW.getTime() - m * 60_000).toISOString();

  it("warns only when collection is on and nothing arrived for 5 minutes", () => {
    expect(flowStale(router({ flowEnabled: false }), NOW.getTime())).toBe(
      false,
    );
    expect(flowStale(router({}), NOW.getTime())).toBe(true);
    expect(
      flowStale(router({ flowLastSeenAt: minutesAgo(2) }), NOW.getTime()),
    ).toBe(false);
    expect(
      flowStale(router({ flowLastSeenAt: minutesAgo(6) }), NOW.getTime()),
    ).toBe(true);
  });
});

describe("peakWindowLabel", () => {
  it("names the step the peak was measured over", () => {
    expect(peakWindowLabel(60)).toBe("1 menit");
    expect(peakWindowLabel(300)).toBe("5 menit");
    expect(peakWindowLabel(3_600)).toBe("1 jam");
    expect(peakWindowLabel(86_400)).toBe("1 hari");
  });
});

function vlan(id: string, comment: string): RouterInterface {
  return {
    id,
    routerId: "r1",
    name: id,
    comment,
    type: "vlan",
    vlanId: null,
    parentName: null,
    ifIndex: null,
    speedMbps: null,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: null,
    lastOutBps: null,
    lastPolledAt: null,
    lastTrafficAt: null,
    silentSince: null,
  };
}

function session(name: string, interfaceId: string | null): RouterBgpSession {
  return { name, interfaceId } as RouterBgpSession;
}

describe("peeringLabels", () => {
  it("names an upstream by its BGP sessions, with the interface after", () => {
    const labels = peeringLabels(
      [vlan("ix", ""), vlan("transit", "Transit A"), vlan("pop", "POP A")],
      [
        session("peer-ix-rs1", "ix"),
        session("peer-ix-rs2", "ix"),
        session("peer-transit", "transit"),
        session("unmatched", null),
      ],
    );

    expect(labels.get("ix")).toBe("peer-ix-rs1, peer-ix-rs2 (ix)");
    expect(labels.get("transit")).toBe("peer-transit (Transit A)");
    expect(labels.get("pop")).toBe("POP A");
  });
});

describe("peeringChartRows", () => {
  it("stacks upstreams by name and aligns their points by time", () => {
    const data: FlowPeering = {
      asn: 64496,
      name: "EXAMPLE-NET-A",
      totalBytes: 0,
      pops: [],
      stepSeconds: 60,
      peakWindowSeconds: 60,
      rows: [],
      series: [
        {
          upstreamInterfaceId: "ix",
          points: [
            { time: "2026-10-12T00:01:00Z", bps: 30 },
            { time: "2026-10-12T00:00:00Z", bps: 10 },
          ],
        },
        {
          upstreamInterfaceId: "transit",
          points: [{ time: "2026-10-12T00:01:00Z", bps: 5 }],
        },
      ],
    };

    const chart = peeringChartRows(
      data,
      new Map([
        ["ix", "IX"],
        ["transit", "Transit"],
      ]),
    );

    expect(chart.series).toEqual(["IX", "Transit"]);
    expect(chart.rows).toEqual([
      { time: "2026-10-12T00:00:00Z", IX: 10, Transit: 0 },
      { time: "2026-10-12T00:01:00Z", IX: 30, Transit: 5 },
    ]);
  });
});

describe("ownPrefixesAvailable", () => {
  it("holds within the 30 days the server keeps", () => {
    expect(
      ownPrefixesAvailable({ kind: "relative", hours: 7 * DAY_HOURS }, NOW),
    ).toBe(true);
    expect(
      ownPrefixesAvailable({ kind: "relative", hours: 30 * DAY_HOURS }, NOW),
    ).toBe(true);
    expect(
      ownPrefixesAvailable({ kind: "relative", hours: 90 * DAY_HOURS }, NOW),
    ).toBe(false);
  });
});
