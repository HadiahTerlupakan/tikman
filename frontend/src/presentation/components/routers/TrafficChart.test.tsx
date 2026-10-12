import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { TrafficChart } from "./TrafficChart";

// jsdom gives the chart no size, so recharts draws nothing; the areas are
// stood in for by elements carrying the props that decide how they draw.
vi.mock("recharts", async (importOriginal) => ({
  ...(await importOriginal<typeof import("recharts")>()),
  ResponsiveContainer: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  AreaChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Area: ({
    dataKey,
    stackId,
    hide,
  }: {
    dataKey: string;
    stackId?: string;
    hide?: boolean;
  }) => (
    <div
      data-testid="area"
      data-key={dataKey}
      data-stack={stackId ?? ""}
      data-hidden={String(Boolean(hide))}
    />
  ),
}));

const DATA = [
  { time: "2026-10-12T00:00:00Z", "Peer A": 100_000_000, "Peer B": 2_000_000 },
  { time: "2026-10-12T00:01:00Z", "Peer A": 300_000_000, "Peer B": 4_000_000 },
];

function renderChart() {
  return render(
    <TrafficChart title="Download" data={DATA} series={["Peer A", "Peer B"]} />,
  );
}

const stacks = () =>
  screen.getAllByTestId("area").map((a) => a.getAttribute("data-stack"));

describe("TrafficChart", () => {
  it("draws each series at its own value until stacking is chosen", () => {
    renderChart();

    expect(screen.getByRole("radio", { name: "Garis" })).toBeChecked();
    expect(stacks()).toEqual(["", ""]);

    fireEvent.click(screen.getByText("Bertumpuk"));
    expect(stacks()).toEqual(["stack", "stack"]);
  });

  it("lists each series' current, average and maximum under the chart", () => {
    renderChart();

    const row = screen.getByText("Peer B").closest("tr") as HTMLElement;
    const cells = within(row)
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells.slice(1)).toEqual(["4.00 Mbps", "3.00 Mbps", "4.00 Mbps"]);
  });

  it("hides and shows a line from its name", () => {
    renderChart();
    const peerA = () =>
      screen
        .getAllByTestId("area")
        .find((a) => a.getAttribute("data-key") === "Peer A");

    fireEvent.click(screen.getByText("Peer A"));
    expect(peerA()).toHaveAttribute("data-hidden", "true");
    expect(screen.getByText("Peer A").closest("[title]")).toHaveAttribute(
      "title",
      "Tampilkan garis",
    );

    fireEvent.click(screen.getByText("Peer A"));
    expect(peerA()).toHaveAttribute("data-hidden", "false");
  });
});
