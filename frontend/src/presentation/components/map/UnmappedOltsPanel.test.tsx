import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import type { Olt } from "@/domain/entities";
import { UnmappedOltsPanel } from "./UnmappedOltsPanel";

const olt = (overrides: Partial<Olt> & { id: string; name: string }): Olt =>
  ({ siteName: "Depok", ipAddress: "192.168.220.22", ...overrides }) as Olt;

const renderPanel = (olts: Olt[]) =>
  render(
    <MemoryRouter>
      <UnmappedOltsPanel olts={olts} />
    </MemoryRouter>,
  );

describe("UnmappedOltsPanel", () => {
  it("says nothing is missing when every OLT has a pin", () => {
    renderPanel([]);

    expect(screen.getByText(/every olt is on the map/i)).toBeInTheDocument();
  });

  it("names the OLTs the map cannot show, and where they are", () => {
    // A map with two pins for three OLTs reads as complete. Naming the gap is
    // the only way an operator learns one is missing rather than absent.
    renderPanel([olt({ id: "o1", name: "OLT Cariu", siteName: "Cariu" })]);

    expect(screen.getByText("OLT Cariu")).toBeInTheDocument();
    expect(screen.getByText(/Cariu · 192\.168\.220\.22/)).toBeInTheDocument();
    expect(screen.getByText(/1 OLT has no coordinates/i)).toBeInTheDocument();
  });

  it("counts more than one correctly", () => {
    renderPanel([olt({ id: "o1", name: "A" }), olt({ id: "o2", name: "B" })]);

    expect(screen.getByText(/2 OLTs have no coordinates/i)).toBeInTheDocument();
  });

  it("points at the page where the coordinates are actually set", () => {
    renderPanel([olt({ id: "o1", name: "A" })]);

    expect(screen.getByRole("link")).toHaveAttribute("href", "/olts");
  });
});
