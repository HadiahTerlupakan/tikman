import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusBar } from "./StatusBar";

describe("StatusBar", () => {
  it("says nothing is registered rather than drawing an empty bar", () => {
    render(
      <StatusBar segments={[]} total={0} emptyText="No ONTs registered yet" />,
    );

    expect(screen.getByText("No ONTs registered yet")).toBeInTheDocument();
  });

  it("keeps an empty bucket in the legend", () => {
    // A visible zero is information: it says LOS was checked and found none.
    render(
      <StatusBar
        total={4}
        emptyText="none"
        segments={[
          { label: "Online", tone: "success", value: 4 },
          { label: "LOS", tone: "danger", value: 0 },
        ]}
      />,
    );

    expect(screen.getByText("LOS")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("shows each bucket's share of the whole", () => {
    render(
      <StatusBar
        total={4}
        emptyText="none"
        segments={[
          { label: "Online", tone: "success", value: 3 },
          { label: "Offline", tone: "neutral", value: 1 },
        ]}
      />,
    );

    expect(screen.getByText("75%")).toBeInTheDocument();
    expect(screen.getByText("25%")).toBeInTheDocument();
  });

  it("renders a dash for a figure it does not know", () => {
    render(
      <StatusBar
        total={4}
        emptyText="none"
        segments={[{ label: "Online", tone: "success", value: null }]}
      />,
    );

    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
  });
});
