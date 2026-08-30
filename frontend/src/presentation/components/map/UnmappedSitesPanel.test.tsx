import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { UnmappedSitesPanel } from "./UnmappedSitesPanel";

// The panel links to /sites, which needs a Router ancestor even in a unit
// test — without it react-router-dom's <Link> throws on render.
const renderPanel = (
  sites: Parameters<typeof UnmappedSitesPanel>[0]["sites"],
) =>
  render(
    <MemoryRouter>
      <UnmappedSitesPanel sites={sites} />
    </MemoryRouter>,
  );

const SITE = {
  id: "s1",
  name: "Gudang",
  location: "Belakang kantor",
  description: "",
  oltCount: 1,
  createdAt: "",
  updatedAt: "",
};

describe("UnmappedSitesPanel", () => {
  it("says nothing is missing when every site has a pin", () => {
    renderPanel([]);

    expect(screen.getByText(/every site is on the map/i)).toBeInTheDocument();
  });

  it("names the sites the map cannot show", () => {
    // A map with two pins for three sites reads as complete. Naming the gap is
    // the only way an operator learns a site is missing rather than absent.
    renderPanel([SITE]);

    expect(screen.getByText("Gudang")).toBeInTheDocument();
    expect(screen.getByText(/1 site has no coordinates/i)).toBeInTheDocument();
  });

  it("counts more than one correctly", () => {
    renderPanel([SITE, { ...SITE, id: "s2", name: "Depok" }]);

    expect(
      screen.getByText(/2 sites have no coordinates/i),
    ).toBeInTheDocument();
  });
});
