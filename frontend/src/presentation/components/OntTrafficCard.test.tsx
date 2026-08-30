import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OntStatus } from "@/domain/entities/Ont";
import { OntTrafficCard } from "./OntTrafficCard";
import type { ReactNode } from "react";

vi.mock("@/application/hooks/useOntMetrics", () => ({
  useOntTrafficTimeSeries: () => ({
    isLoading: false,
    // The endpoint returns the consolidated points together with the volume
    // moved over the window, which the rates alone cannot give.
    data: {
      points: [
        {
          time: "2026-08-21T09:55:00.000Z",
          txMbps: 1,
          rxMbps: 2,
          txMaxMbps: 12,
          rxMaxMbps: 8,
        },
      ],
      usage: { downloadBytes: 1_500_000_000, uploadBytes: 250_000_000 },
    },
  }),
}));

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  AreaChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Area: () => null,
  CartesianGrid: () => null,
  Legend: () => null,
  Tooltip: () => null,
  YAxis: () => null,
  XAxis: ({
    domain,
    ticks,
    tickFormatter,
  }: {
    domain?: [number, number];
    ticks?: number[];
    tickFormatter?: (value: number) => string;
  }) => (
    <div
      data-testid="x-axis"
      data-domain={JSON.stringify(domain)}
      data-ticks={JSON.stringify(ticks)}
      data-labels={JSON.stringify(ticks?.map((t) => tickFormatter?.(t)))}
    />
  ),
}));

const ONT = {
  id: "ont-1",
  oltId: "olt-1",
  oltName: "OLT 1",
  slot: 1,
  portId: 1,
  ontId: 1,
  serialNumber: "RTEGC609833D",
  name: "Customer",
  description: "",
  status: OntStatus.ONLINE,
  lastSeenAt: null,
  createdAt: "2026-08-21T00:00:00.000Z",
  updatedAt: "2026-08-21T00:00:00.000Z",
};

afterEach(() => {
  vi.useRealTimers();
});

describe("OntTrafficCard", () => {
  it("names the OLT the graph belongs to", () => {
    // A search spans every OLT, so a screen of nine graphs can mix sites. The
    // serial alone does not say which one an operator is looking at.
    render(<OntTrafficCard ont={ONT} period="3h" />);

    expect(screen.getByText("OLT 1")).toBeInTheDocument();
  });

  it("omits the label rather than showing an empty tag", () => {
    render(<OntTrafficCard ont={{ ...ONT, oltName: "" }} period="3h" />);

    expect(screen.getByText(/RTEGC609833D/)).toBeInTheDocument();
    expect(screen.queryByText("OLT 1")).not.toBeInTheDocument();
  });

  it("uses the full selected period as the default chart domain", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-21T10:00:00.000Z"));

    render(<OntTrafficCard ont={ONT} period="3h" />);

    expect(
      JSON.parse(screen.getByTestId("x-axis").dataset.domain || "[]"),
    ).toEqual([
      new Date("2026-08-21T07:00:00.000Z").getTime(),
      new Date("2026-08-21T10:00:00.000Z").getTime(),
    ]);
  });

  it("spans axis ticks across the whole custom range, not just where data exists", () => {
    const start = new Date("2026-07-01T00:00:00.000Z").getTime();
    const end = new Date("2026-08-31T16:59:59.000Z").getTime();

    render(
      <OntTrafficCard
        ont={ONT}
        period="3h"
        range={{
          start: "2026-07-01T00:00:00.000Z",
          end: "2026-08-31T16:59:59.000Z",
          bucket: "hour",
        }}
      />,
    );

    // The single data point sits on Aug 21, yet ticks must still reach both
    // ends of the selected range so the empty weeks stay visible.
    const ticks: number[] = JSON.parse(
      screen.getByTestId("x-axis").dataset.ticks || "[]",
    );
    expect(ticks).toHaveLength(5);
    expect(ticks[0]).toBe(start);
    expect(ticks[4]).toBe(end);

    const labels: string[] = JSON.parse(
      screen.getByTestId("x-axis").dataset.labels || "[]",
    );
    expect(labels.some((label) => label.startsWith("Jul"))).toBe(true);
  });

  it("reports Maximum from the per-bucket peak, not the averaged value", () => {
    render(<OntTrafficCard ont={ONT} period="3h" />);

    // Download peak = txMaxMbps (12 Mbps), Upload peak = rxMaxMbps (8 Mbps).
    expect(screen.getByText("Maximum: 12.00 Mbps")).toBeInTheDocument();
    expect(screen.getByText("Maximum: 8.00 Mbps")).toBeInTheDocument();
  });
});

describe("OntTrafficCard totals", () => {
  it("reports the volume moved over the window, not only the rates", () => {
    render(
      <OntTrafficCard
        ont={
          {
            id: "ont-1",
            serialNumber: "HWTC1",
            status: OntStatus.ONLINE,
          } as never
        }
        period="3h"
      />,
    );

    expect(screen.getByText("Total: 1.40 GB")).toBeInTheDocument();
    expect(screen.getByText("Total: 238.42 MB")).toBeInTheDocument();
  });
});
