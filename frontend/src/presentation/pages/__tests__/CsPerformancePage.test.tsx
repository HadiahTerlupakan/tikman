import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";

const fixtures = vi.hoisted(() => {
  const stats = (
    count: number,
    median: number | null,
    pct: number | null,
    p90: number | null,
  ) => ({
    count,
    medianMinutes: median,
    withinTargetPct: pct,
    p90Minutes: p90,
  });

  return {
    queries: [] as unknown[],
    summary: {
      targetMinutes: 15,
      team: {
        replies: stats(12, 3.2, 83, 30.8),
        closedWithoutReply: 2,
        abandoned: 1,
        systemDelayed: 4,
      },
      agents: [
        {
          userId: "u-ani",
          username: "ani",
          replies: stats(7, 2.3, 89, 17.9),
          closedWithoutReply: 0,
        },
        {
          userId: "u-budi",
          username: "budi",
          replies: stats(0, null, null, null),
          closedWithoutReply: 2,
        },
      ],
      days: [{ date: "2026-09-16", replies: stats(12, 3.2, 83, 30.8) }],
      waiting: { count: 3, longestMinutes: 18 },
    },
    waits: {
      items: [
        {
          id: "w1",
          conversationId: "c1",
          customerName: "Pak Budi",
          customerPhone: "628123456789",
          conversationDeleted: false,
          startedAt: "2026-09-16T01:00:00Z",
          customerSentAt: "2026-09-16T01:00:00Z",
          endedAt: "2026-09-16T01:05:00Z",
          endReason: "replied",
          endedBy: "u-ani",
          endedByUsername: "ani",
          teamMinutes: 5,
          countedMinutes: 5,
          systemDelayed: false,
        },
        {
          id: "w2",
          conversationId: "c2",
          customerName: "",
          customerPhone: "",
          conversationDeleted: true,
          startedAt: "2026-09-16T02:00:00Z",
          customerSentAt: "2026-09-16T01:00:00Z",
          endedAt: "2026-09-16T02:10:00Z",
          endReason: "closed",
          endedBy: "u-ani",
          endedByUsername: "ani",
          teamMinutes: null,
          countedMinutes: null,
          systemDelayed: true,
        },
      ],
      total: 2,
    },
  };
});

vi.mock("@/application/hooks/useCsPerformance", () => ({
  useCsPerformanceSummary: () => ({ data: fixtures.summary, isLoading: false }),
  useCsPerformanceWaits: (query?: unknown) => {
    fixtures.queries.push(query);
    return { data: query ? fixtures.waits : undefined, isLoading: false };
  },
}));

// jsdom has no layout, so recharts warns about a zero-sized chart on every
// render. The chart is not what these tests are about.
vi.mock("../../components/cs/performance/DailyPerformanceChart", () => ({
  DailyPerformanceChart: () => null,
}));

import { CsPerformancePage } from "../CsPerformancePage";

function draw() {
  return render(
    <MemoryRouter>
      <CsPerformancePage />
    </MemoryRouter>,
  );
}

const lastQuery = () =>
  fixtures.queries[fixtures.queries.length - 1] as Record<string, unknown>;

describe("the CS performance page", () => {
  it("shows the team's figures and who is waiting now", () => {
    draw();

    expect(screen.getByText("83%")).toBeInTheDocument();
    expect(screen.getByText("3,2 mnt")).toBeInTheDocument();
    expect(screen.getByText(/Ditutup tanpa balasan: 2/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: /Sedang menunggu: 3 pelanggan, terlama 18 mnt/,
      }),
    ).toHaveAttribute("href", "/cs?view=belum-dibalas");
  });

  it("gives every CS a row, with dashes where there is nothing to measure", () => {
    draw();

    const budi = screen.getByText("budi").closest("tr");
    expect(budi).not.toBeNull();
    expect(within(budi!).getByText("2")).toBeInTheDocument();
    expect(within(budi!).getAllByText("—").length).toBeGreaterThanOrEqual(3);
    expect(screen.getByText("2,3 mnt")).toBeInTheDocument();
  });

  it("opens one CS's waits, marking a deleted thread and a system delay", async () => {
    draw();

    await userEvent.click(screen.getByText("ani"));

    expect(screen.getByText("Giliran ani")).toBeInTheDocument();
    expect(screen.getByText("Pak Budi")).toBeInTheDocument();
    expect(screen.getByText("Thread dihapus")).toBeInTheDocument();
    expect(screen.getByText("Tertunda sistem")).toBeInTheDocument();
    expect(lastQuery()).toMatchObject({
      userId: "u-ani",
      limit: 20,
      offset: 0,
    });
  });

  it("lists the whole team's waits when no CS is named", async () => {
    draw();

    await userEvent.click(
      screen.getByRole("button", { name: "Daftar giliran" }),
    );

    // The trigger button carries the same label as the whole-team drawer's
    // title, and the drawer overlays the page rather than replacing it, so an
    // unscoped query would match both. Scoping to the dialog is what
    // disambiguates a title from the button that opened it.
    expect(
      within(screen.getByRole("dialog")).getByText("Daftar giliran"),
    ).toBeInTheDocument();
    expect(lastQuery().userId).toBeUndefined();
  });
});
