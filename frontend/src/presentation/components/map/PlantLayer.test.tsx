import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Odc, Odp } from "@/domain/entities";
import { PlantLayer } from "./PlantLayer";

// The real Marker talks to Google's script, which cannot load under jsdom.
// Recording the icon keeps the one thing the pin carries — how full the box is
// — observable, since that is the whole point of drawing it.
vi.mock("@vis.gl/react-google-maps", () => ({
  Marker: ({
    title,
    icon,
    onClick,
  }: {
    title: string;
    icon: string;
    onClick: () => void;
  }) => (
    <button type="button" data-icon={icon} onClick={onClick}>
      {title}
    </button>
  ),
  InfoWindow: ({ children }: { children: ReactNode }) => (
    <div data-testid="info">{children}</div>
  ),
}));

const cabinet: Odc = {
  id: "odc-1",
  siteId: "site-1",
  name: "ODC Cariu 1",
  code: "ODC-CRU-01",
  latitude: -6.4,
  longitude: 106.8,
  address: "",
  notes: "",
  feedCount: 2,
  odpCount: 5,
};

const box = (over: Partial<Odp> = {}): Odp => ({
  id: "odp-1",
  name: "ODP-01",
  code: "",
  portCount: 8,
  usedPorts: 0,
  latitude: -6.41,
  longitude: 106.81,
  address: "",
  notes: "",
  odcId: "odc-1",
  ...over,
});

function colorOf(name: string | RegExp): string {
  return screen.getByRole("button", { name }).getAttribute("data-icon") ?? "";
}

describe("PlantLayer", () => {
  it("says how full a box is on the pin itself", () => {
    render(<PlantLayer odcs={[]} odps={[box({ usedPorts: 3 })]} />);

    // Hovering a pin should answer the question without opening anything.
    expect(
      screen.getByRole("button", { name: /ODP-01 — 3\/8 port terpakai/ }),
    ).toBeInTheDocument();
  });

  it("draws a full box differently from one with room", () => {
    render(
      <PlantLayer
        odcs={[]}
        odps={[
          box({ id: "a", name: "ODP-A", usedPorts: 1 }),
          box({ id: "b", name: "ODP-B", usedPorts: 8 }),
        ]}
      />,
    );

    expect(colorOf(/ODP-A/)).not.toBe(colorOf(/ODP-B/));
  });

  it("leaves out plant with no coordinates", () => {
    render(
      <PlantLayer
        odcs={[]}
        odps={[
          box({ id: "a", name: "ODP-A" }),
          box({ id: "b", name: "ODP-B", latitude: undefined }),
        ]}
      />,
    );

    // Half a coordinate is not a location, and a pin claiming otherwise puts a
    // technician on the equator.
    expect(screen.getByRole("button", { name: /ODP-A/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /ODP-B/ })).toBeNull();
  });

  it("opens a cabinet's fan-out when its pin is clicked", async () => {
    render(<PlantLayer odcs={[cabinet]} odps={[]} />);

    await userEvent.click(screen.getByRole("button", { name: "ODC Cariu 1" }));

    expect(screen.getByTestId("info")).toHaveTextContent("2 feed · 5 ODP");
  });

  it("offers the subscribers on a box only when someone can show them", async () => {
    const onSelectOdp = vi.fn();
    render(<PlantLayer odcs={[]} odps={[box()]} onSelectOdp={onSelectOdp} />);

    await userEvent.click(screen.getByRole("button", { name: /ODP-01/ }));
    await userEvent.click(
      screen.getByRole("button", { name: "Lihat pelanggan" }),
    );

    expect(onSelectOdp).toHaveBeenCalledWith(
      expect.objectContaining({ id: "odp-1" }),
    );
  });
});
