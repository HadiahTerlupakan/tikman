import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AsPathDiagram } from "./AsPathDiagram";
import { buildPathGraph } from "./asPathGraph";

const graph = buildPathGraph([
  {
    prefix: "198.51.100.0/24",
    peer: "peer-ix",
    asPath: "64500,64496",
    active: true,
    bytes: 100,
  },
  {
    prefix: "198.51.100.0/24",
    peer: "peer-transit",
    asPath: "64501,64501,64502,64496",
    active: false,
    bytes: 100,
  },
]);

function renderDiagram(onSelect = vi.fn()) {
  const utils = render(
    <AsPathDiagram
      graph={graph}
      names={{ "64500": "IX-RS", "64496": "EXAMPLE-NET" }}
      routerLabel="Border"
      onSelect={onSelect}
    />,
  );
  return { ...utils, onSelect };
}

describe("AsPathDiagram", () => {
  it("labels every AS with its number and name, and the router by name", () => {
    renderDiagram();

    expect(screen.getByText("Border")).toBeInTheDocument();
    expect(screen.getByText("AS64500")).toBeInTheDocument();
    expect(screen.getByText("IX-RS")).toBeInTheDocument();
    expect(screen.getAllByText("AS64496")).toHaveLength(2);
    expect(screen.getByText("AS64502")).toBeInTheDocument();
  });

  it("draws the active path solid and the alternative dashed", () => {
    const { container } = renderDiagram();

    const edges = container.querySelectorAll("path[data-edge]");
    expect(edges).toHaveLength(5);
    const active = container.querySelectorAll(
      'path[data-edge][data-active="true"]',
    );
    expect(active).toHaveLength(2);
    expect(active[0].getAttribute("stroke-dasharray")).toBeNull();
    const dashed = container.querySelector(
      'path[data-edge][data-active="false"]',
    );
    expect(dashed?.getAttribute("stroke-dasharray")).not.toBeNull();
  });

  it("notes prepends on the AS that padded its path", () => {
    renderDiagram();

    expect(screen.getByText("+1 prepend")).toBeInTheDocument();
  });

  it("reports the node clicked", () => {
    const { onSelect } = renderDiagram();

    fireEvent.click(screen.getByText("AS64502"));

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: "2:64502", asn: 64502 }),
    );
  });

  it("says so when there is no path to draw", () => {
    render(
      <AsPathDiagram
        graph={buildPathGraph([])}
        names={{}}
        routerLabel="Border"
      />,
    );

    expect(
      screen.getByText("Belum ada jalur untuk digambar."),
    ).toBeInTheDocument();
  });
});
