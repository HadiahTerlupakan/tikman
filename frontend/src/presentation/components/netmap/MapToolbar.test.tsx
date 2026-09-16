import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MapToolbar } from "./MapToolbar";

const noop = () => {};

describe("MapToolbar", () => {
  it("offers one button per kind of thing that goes on a map", () => {
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    expect(screen.getByRole("button", { name: /Server/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODC/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODP/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ONT/ })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Tarik kabel/ }),
    ).toBeInTheDocument();
  });

  it("says which kind is being placed", async () => {
    const onPlace = vi.fn();
    render(
      <MapToolbar
        placing={undefined}
        onPlace={onPlace}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /ODP/ }));

    expect(onPlace).toHaveBeenCalledWith("odp");
  });

  // While a box is being placed the other kinds are noise; what is needed is a
  // way out.
  it("offers a way to cancel once placing has started", async () => {
    const onCancel = vi.fn();
    render(
      <MapToolbar
        placing="odp"
        onPlace={noop}
        onDrawCable={noop}
        onCancel={onCancel}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Batal/ }));

    expect(onCancel).toHaveBeenCalled();
  });
});
