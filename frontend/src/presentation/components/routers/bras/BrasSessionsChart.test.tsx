import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import dayjs from "dayjs";
import type { BrasSample } from "@/domain/entities";
import { BrasSessionsChart } from "./BrasSessionsChart";
import { sessionsSeries } from "./brasSeries";

// jsdom gives the chart no size, so recharts draws nothing; the parts under
// test are stood in for by elements carrying the props that decide them.
vi.mock("recharts", async (importOriginal) => ({
  ...(await importOriginal<typeof import("recharts")>()),
  ResponsiveContainer: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  LineChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  ReferenceDot: ({ label }: { label?: { value: string } }) => (
    <div data-testid="drop">{label?.value}</div>
  ),
}));

function sample(time: string, sessions: number): BrasSample {
  return {
    time,
    sessions,
    cpuMax: 10,
    cpuAvg: 5,
    radiusAccepts: 0,
    radiusRejects: 0,
    radiusTimeouts: 0,
    poolMinFree: null,
  };
}

const SAMPLES = [
  sample("2026-11-20T10:00:00Z", 1800),
  sample("2026-11-20T10:01:00Z", 1810),
  sample("2026-11-20T10:04:00Z", 1500),
  sample("2026-11-20T10:05:00Z", 1490),
];

describe("sessionsSeries", () => {
  it("breaks the line where samples are more than 2 minutes apart", () => {
    const sessions = sessionsSeries(SAMPLES).map((p) => p.sessions);
    expect(sessions).toEqual([1800, 1810, null, 1500, 1490]);
  });

  it("joins samples exactly 2 minutes apart", () => {
    const series = sessionsSeries([
      sample("2026-11-20T10:00:00Z", 1800),
      sample("2026-11-20T10:02:00Z", 1790),
    ]);
    expect(series.map((p) => p.sessions)).toEqual([1800, 1790]);
  });
});

describe("BrasSessionsChart", () => {
  it("labels each drop with its time and size", () => {
    const drop = { time: "2026-11-20T10:05:00Z", lost: 312 };
    render(<BrasSessionsChart samples={SAMPLES} drops={[drop]} />);
    expect(screen.getByTestId("drop")).toHaveTextContent(
      `${dayjs(drop.time).format("HH:mm")} turun 312 sesi`,
    );
  });

  it("says when there is nothing to chart", () => {
    render(<BrasSessionsChart samples={[]} drops={[]} />);
    expect(screen.getByText("Belum ada data di rentang ini")).toBeVisible();
  });
});
