import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MapToolbar } from "./MapToolbar";

// The export button's own download mechanics (useExportMapping, downloadFile)
// are exercised by MappingRepository.test.ts and downloadFile.test.ts; this
// file only needs to know MapToolbar asks for the mutation and hands its
// result off, without a real QueryClient to back a live useMutation.
const mutateAsync = vi.fn();
vi.mock("@/application/hooks", () => ({
  useExportMapping: () => ({ mutateAsync, isPending: false }),
}));
vi.mock("./downloadFile", () => ({ downloadFile: vi.fn() }));
// antd's message is an imperative singleton, not something a render finds in
// the DOM reliably under jsdom - spying on it directly is what lets a click
// assert whether the warning toast fired, without pulling in a real overlay.
vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    message: { ...actual.message, warning: vi.fn(), error: vi.fn() },
  };
});

const noop = () => {};

describe("MapToolbar", () => {
  // message.warning's call history otherwise survives between tests (it is
  // the same mock function for the whole file) - the negative assertion
  // below ("says nothing extra") is exactly the case that would silently
  // pass or fail depending on what an earlier test happened to trigger.
  beforeEach(() => vi.clearAllMocks());

  // Hand-placing a "server" is the duplicate-OLT concept this feature
  // removes; a real OLT goes on the map through its own dedicated button.
  it("offers one button per kind of thing that goes on a map, and OLT instead of hand-placed Server", () => {
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    expect(screen.getByRole("button", { name: /OLT/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODC/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODP/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ONT/ })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Tarik kabel/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Server/ }),
    ).not.toBeInTheDocument();
  });

  it("says which kind is being placed", async () => {
    const onPlace = vi.fn();
    render(
      <MapToolbar
        placing={undefined}
        onPlace={onPlace}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /ODP/ }));

    expect(onPlace).toHaveBeenCalledWith("odp");
  });

  // A distinct callback, not a NodeType value through onPlace: placing an
  // OLT does not create a new mapping row the way every other button does —
  // it picks an existing OLT and writes a position onto it (OltPlacementModal).
  it("arms OLT placement through its own callback, not onPlace", async () => {
    const onPlace = vi.fn();
    const onPlaceOlt = vi.fn();
    render(
      <MapToolbar
        placing={undefined}
        onPlace={onPlace}
        onPlaceOlt={onPlaceOlt}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /OLT/ }));

    expect(onPlaceOlt).toHaveBeenCalled();
    expect(onPlace).not.toHaveBeenCalled();
  });

  // While a box is being placed the other kinds are noise; what is needed is a
  // way out.
  it("offers a way to cancel once placing has started", async () => {
    const onCancel = vi.fn();
    render(
      <MapToolbar
        placing="odp"
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={onCancel}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Batal/ }));

    expect(onCancel).toHaveBeenCalled();
  });

  // Arming OLT placement is a placing mode like any other: it must give the
  // same way out, not leave the picker button sitting there mid-flow.
  it("offers the same cancel button while OLT placement is armed", () => {
    render(
      <MapToolbar
        placing="olt"
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    expect(screen.getByRole("button", { name: /Batal/ })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /OLT/ }),
    ).not.toBeInTheDocument();
  });

  // Small and unobtrusive: available next to the Peta/Daftar toggle
  // regardless of what is being placed, since exporting does not conflict
  // with an in-progress placement.
  it("downloads the map as a KMZ when asked", async () => {
    const { downloadFile } = await import("./downloadFile");
    const blob = new Blob(["fake kmz"]);
    mutateAsync.mockResolvedValue({ blob, warning: "" });
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Unduh KMZ/ }));

    expect(mutateAsync).toHaveBeenCalled();
    expect(downloadFile).toHaveBeenCalledWith(blob, "peta-jaringan.kmz");
  });

  // A cable dropped for a missing endpoint must not disappear silently - the
  // backend names how many and why in one sentence; this only has to relay it.
  it("warns when the export left cables out", async () => {
    const { message } = await import("antd");
    const warning =
      "1 kabel dilewati karena salah satu ujungnya sudah dihapus dari peta";
    mutateAsync.mockResolvedValue({ blob: new Blob(["fake kmz"]), warning });
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Unduh KMZ/ }));

    expect(message.warning).toHaveBeenCalledWith(warning);
  });

  // The other half of the same rule: a one-sided test that only checks the
  // warning fires would still pass a version that always warns regardless of
  // what the export actually did.
  it("says nothing extra when the export is clean", async () => {
    const { message } = await import("antd");
    mutateAsync.mockResolvedValue({
      blob: new Blob(["fake kmz"]),
      warning: "",
    });
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onPlaceOlt={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Unduh KMZ/ }));

    expect(message.warning).not.toHaveBeenCalled();
  });
});
