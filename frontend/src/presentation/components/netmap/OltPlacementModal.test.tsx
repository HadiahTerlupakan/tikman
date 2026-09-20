import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Olt } from "@/domain/entities";
import { OltPlacementModal } from "./OltPlacementModal";

function olt(overrides: Partial<Olt>): Olt {
  return {
    id: "olt-1",
    name: "OLT-01",
    siteName: "Site Satu",
    ...overrides,
  } as Olt;
}

describe("OltPlacementModal", () => {
  it("shows where the map was tapped", () => {
    render(
      <OltPlacementModal
        open
        position={{ lat: -6.21, lng: 106.81 }}
        olts={[olt({ id: "1" })]}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByText("-6.210000, 106.810000")).toBeInTheDocument();
  });

  it("offers only the OLTs it was given, not a hardcoded list", async () => {
    const user = userEvent.setup();
    render(
      <OltPlacementModal
        open
        position={{ lat: -6.21, lng: 106.81 }}
        olts={[
          olt({ id: "1", name: "OLT-01", siteName: "Site Satu" }),
          olt({ id: "2", name: "OLT-02", siteName: "Site Dua" }),
        ]}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    await user.click(screen.getByRole("combobox"));

    expect(await screen.findByTitle("OLT-01 — Site Satu")).toBeInTheDocument();
    expect(screen.getByTitle("OLT-02 — Site Dua")).toBeInTheDocument();
  });

  it("will not save without an OLT chosen", async () => {
    const onSubmit = vi.fn();
    render(
      <OltPlacementModal
        open
        position={{ lat: -6.21, lng: 106.81 }}
        olts={[olt({ id: "1" })]}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits the chosen OLT's id", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <OltPlacementModal
        open
        position={{ lat: -6.21, lng: 106.81 }}
        olts={[olt({ id: "olt-42", name: "OLT-42", siteName: "Site Satu" })]}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByRole("combobox"));
    await user.click(await screen.findByTitle("OLT-42 — Site Satu"));
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith("olt-42");
  });

  it("cancels without submitting anything", async () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn();
    render(
      <OltPlacementModal
        open
        position={{ lat: -6.21, lng: 106.81 }}
        olts={[olt({ id: "1" })]}
        onCancel={onCancel}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Batal" }));

    expect(onCancel).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
