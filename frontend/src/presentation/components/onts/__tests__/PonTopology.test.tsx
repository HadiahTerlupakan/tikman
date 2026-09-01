import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { PonHealth } from "@/domain/entities";
import { PonTopology } from "../PonTopology";

const health: PonHealth = {
  oltId: "olt-1",
  oltName: "Cariu",
  medianTrapPerOnt: 19,
  trapThreshold: 100,
  outageThreshold: 0.05,
  cards: [
    {
      slot: 8,
      ponCount: 1,
      pons: [
        {
          port: 12,
          ontCount: 41,
          trapPerOnt: 686,
          outageShare: 0.12,
          worst: [
            {
              ontId: "ont-1",
              label: "ONU-8:12",
              name: "MAD SURYA",
              trapCount: 1204,
              downMinutes: 340,
            },
          ],
        },
      ],
    },
  ],
};

describe("PonTopology", () => {
  it("draws the branch down to the subscriber", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(screen.getByText("Cariu")).toBeInTheDocument();
    expect(screen.getByText("Kartu 8")).toBeInTheDocument();
    expect(screen.getByText("PON 12")).toBeInTheDocument();
    expect(screen.getByText("ONU-8:12")).toBeInTheDocument();
  });

  it("carries every figure as text, not only as colour", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(screen.getByText(/686 trap\/ONT · 12% mati/)).toBeInTheDocument();
  });

  it("hands the chosen port back to its caller", () => {
    const onSelectPon = vi.fn();
    render(<PonTopology health={health} onSelectPon={onSelectPon} />);

    fireEvent.click(screen.getByText("PON 12"));

    expect(onSelectPon).toHaveBeenCalledWith(8, 12);
  });

  it("shows the rule it applied instead of applying it invisibly", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(
      screen.getByText(/di atas 100 dan lima kali median/i),
    ).toBeInTheDocument();
  });

  it("says the OLT is healthy rather than drawing an empty canvas", () => {
    render(
      <PonTopology health={{ ...health, cards: [] }} onSelectPon={vi.fn()} />,
    );

    expect(screen.getByText(/tidak ada PON bermasalah/i)).toBeInTheDocument();
  });
});
