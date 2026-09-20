import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Waypoint } from "@/domain/entities";
import { ApiError } from "@/infrastructure/http";
import { NetworkMapPage } from "../NetworkMapPage";

// Split out of the other NetworkMapPage.*.test.tsx files for the same reason
// they are already split: placing an OLT is its own surface (arming,
// picking, and the two error codes unique to this path) big enough to keep
// under the project's line limit. The mock setup is a full copy rather than
// a shared import — vi.mock factories are hoisted above this file's own
// imports, so a value imported from a sibling fixtures module is still in
// its temporal dead zone the first time such a factory runs.

// olt-2 already has coordinates; only olt-1 is a candidate to place. A plain
// object (not vi.hoisted) reset in beforeEach, since useOltsMock below —
// not this fixture — is what a test overrides to reach the empty-fleet case.
const unplacedAndPlacedOlts = [
  { id: "olt-1", name: "OLT-01", siteName: "Site Satu" },
  {
    id: "olt-2",
    name: "OLT-02",
    siteName: "Site Dua",
    latitude: -6.3,
    longitude: 106.9,
  },
];

const useOltsMock = vi.hoisted(() => vi.fn());
const updateOltMutateAsync = vi.hoisted(() => vi.fn());
const refetchNodes = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useMappingNodes: () => ({
    data: [],
    isLoading: false,
    refetch: refetchNodes,
  }),
  useMappingEdges: () => ({ data: [], isLoading: false }),
  useOlts: useOltsMock,
  useUpdateOlt: () => ({
    mutateAsync: updateOltMutateAsync,
    isPending: false,
  }),
  useCreateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useGoogleMapsKey: () => ({
    key: "test-key",
    mapId: "test-map",
    isLoading: false,
  }),
  // MapToolbar's own export button; its download mechanics are covered by
  // MapToolbar.test.tsx, downloadFile.test.ts and MappingRepository.test.ts.
  useExportMapping: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

interface MockCanvasProps {
  onDrop: (point: Waypoint) => void;
}

let canvasProps: MockCanvasProps;

vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: (props: MockCanvasProps) => {
    canvasProps = props;
    return <div data-testid="map-canvas" />;
  },
}));

const messageSuccess = vi.hoisted(() => vi.fn());
const messageError = vi.hoisted(() => vi.fn());
const messageInfo = vi.hoisted(() => vi.fn());

vi.mock("antd", async () => {
  const antd = await vi.importActual<typeof import("antd")>("antd");
  return {
    ...antd,
    message: {
      success: messageSuccess,
      error: messageError,
      info: messageInfo,
    },
  };
});

async function armAndDrop() {
  render(<NetworkMapPage />);
  await userEvent.click(screen.getByRole("button", { name: /\+ OLT/ }));
  act(() => canvasProps.onDrop({ lat: -6.21, lng: 106.81 }));
}

async function pickAndSubmit(label: string) {
  await userEvent.click(screen.getByRole("combobox"));
  await userEvent.click(await screen.findByTitle(label));
  await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
}

describe("NetworkMapPage OLT placement", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useOltsMock.mockReturnValue({
      data: unplacedAndPlacedOlts,
      isLoading: false,
    });
  });

  it("offers only OLTs without coordinates yet", async () => {
    await armAndDrop();
    await userEvent.click(screen.getByRole("combobox"));

    expect(await screen.findByTitle("OLT-01 — Site Satu")).toBeInTheDocument();
    expect(screen.queryByTitle("OLT-02 — Site Dua")).not.toBeInTheDocument();
  });

  // Requirement: an empty picker must never open — the operator is told
  // plainly instead, before any modal exists to be empty.
  it("says plainly that nothing is left to place when every OLT already has coordinates", async () => {
    useOltsMock.mockReturnValue({
      data: [{ id: "olt-2", name: "OLT-02", latitude: -6.3, longitude: 106.9 }],
      isLoading: false,
    });
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /\+ OLT/ }));

    expect(messageInfo).toHaveBeenCalledWith(
      "Semua OLT sudah punya posisi di peta",
    );
    expect(
      screen.queryByRole("button", { name: /Batal/ }),
    ).not.toBeInTheDocument();
  });

  it("arms placement and opens the picker positioned at the map tap", async () => {
    await armAndDrop();

    expect(screen.getByText("-6.210000, 106.810000")).toBeInTheDocument();
  });

  it("writes the tapped position onto the chosen OLT", async () => {
    await armAndDrop();
    await pickAndSubmit("OLT-01 — Site Satu");

    expect(updateOltMutateAsync).toHaveBeenCalledWith({
      id: "olt-1",
      data: { latitude: -6.21, longitude: 106.81 },
    });
  });

  // useUpdateOlt only knows to invalidate its own OLT queries; the mirror
  // node this write creates lives in mapping's own query, which nothing else
  // has a reason to refetch.
  it("refetches the map's nodes after placing an OLT, so its new mirror appears", async () => {
    await armAndDrop();
    await pickAndSubmit("OLT-01 — Site Satu");

    await waitFor(() => expect(refetchNodes).toHaveBeenCalled());
  });

  it("confirms the placement and closes the picker", async () => {
    await armAndDrop();
    await pickAndSubmit("OLT-01 — Site Satu");

    await waitFor(() =>
      expect(messageSuccess).toHaveBeenCalledWith("OLT ditempatkan di peta"),
    );
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("shows an Indonesian message for coordinates the OLT record itself refuses", async () => {
    updateOltMutateAsync.mockRejectedValueOnce(
      new ApiError(
        400,
        "INVALID_COORDINATES",
        undefined,
        "latitude 200 is outside -90..90",
      ),
    );
    await armAndDrop();
    await pickAndSubmit("OLT-01 — Site Satu");

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith("Koordinat tidak valid"),
    );
  });

  it("falls back to a generic message when placing an OLT fails for another reason", async () => {
    updateOltMutateAsync.mockRejectedValueOnce(new Error("network down"));
    await armAndDrop();
    await pickAndSubmit("OLT-01 — Site Satu");

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith(
        "Gagal menempatkan OLT di peta",
      ),
    );
  });
});
