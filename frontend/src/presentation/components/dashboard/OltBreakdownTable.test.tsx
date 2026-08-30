import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { OltStatus } from "@/domain/entities";
import type { OltBreakdown } from "@/presentation/pages/dashboardStats";
import { OltBreakdownTable } from "./OltBreakdownTable";

const depok: OltBreakdown = {
  oltId: "o1",
  oltName: "Depok",
  oltStatus: OltStatus.ONLINE,
  ontTotal: 4,
  online: 3,
  impaired: 1,
  availability: 75,
};

describe("OltBreakdownTable", () => {
  it("invites registration when there is no hardware yet", () => {
    render(<OltBreakdownTable rows={[]} />);

    expect(screen.getByText("No OLTs registered yet")).toBeInTheDocument();
  });

  it("shows where the impaired ONTs actually are", () => {
    render(<OltBreakdownTable rows={[depok]} />);

    expect(screen.getByText("Depok")).toBeInTheDocument();
    expect(screen.getByText("4 ONTs")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("75%")).toBeInTheDocument();
  });

  it("renders a dash rather than 0% for an OLT with nothing to measure", () => {
    render(
      <OltBreakdownTable
        rows={[
          {
            ...depok,
            oltName: "Bekasi",
            ontTotal: 0,
            online: 0,
            impaired: 0,
            availability: null,
          },
        ]}
      />,
    );

    expect(screen.getByText("no ONTs")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
