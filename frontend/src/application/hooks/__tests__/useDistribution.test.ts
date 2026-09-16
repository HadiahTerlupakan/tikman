import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import {
  useAssignOntToOdp,
  useOdpSubscribers,
  useOdps,
  useUnassignOntFromOdp,
} from "../useDistribution";

vi.mock("@/infrastructure/http/apiClient", () => ({
  apiClient: { get: vi.fn(), put: vi.fn(), delete: vi.fn(), post: vi.fn() },
}));

const { apiClient } = await import("@/infrastructure/http/apiClient");

function wrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

describe("useOdpSubscribers", () => {
  beforeEach(() => vi.clearAllMocks());

  // The /odps/:id/subscribers route was deleted with the rest of the
  // distribution package (Task 6), so this now has to ask the general ONT
  // list for one box's occupants. apiClient decamelizes request bodies but
  // never query strings, so an odpId spelled in camelCase here would reach
  // the server unrecognised and silently come back with every ONT instead of
  // one box's — the failure is a wrong answer, not a thrown error.
  it("asks the ONT list for this box by its wire parameter, not a camelCase one", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { data: [] },
    } as never);

    renderHook(() => useOdpSubscribers("odp-1"), { wrapper: wrapper() });

    await waitFor(() =>
      expect(apiClient.get).toHaveBeenCalledWith("/api/v1/onts", {
        params: { odp_id: "odp-1" },
      }),
    );
  });

  it("asks nothing before a box is chosen", () => {
    renderHook(() => useOdpSubscribers(undefined), { wrapper: wrapper() });

    expect(apiClient.get).not.toHaveBeenCalled();
  });
});

describe("useOdps", () => {
  beforeEach(() => vi.clearAllMocks());

  // The /odps listing was deleted with the rest of the distribution package;
  // the box list now comes from the map's own nodes, narrowed to the ones
  // that are distribution boxes.
  it("lists only the mapping nodes that are distribution boxes", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: {
        data: [
          {
            id: "n1",
            nodeId: "ODC-01",
            type: "odc",
            name: "ODC Satu",
            capacity: 0,
          },
          {
            id: "n2",
            nodeId: "ODP-01",
            type: "odp",
            name: "ODP Satu",
            capacity: 8,
          },
        ],
      },
    } as never);

    const { result } = renderHook(() => useOdps(), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data).toHaveLength(1));
    expect(apiClient.get).toHaveBeenCalledWith("/api/v1/mapping/nodes");
    expect(result.current.data?.[0]).toMatchObject({
      id: "n2",
      code: "ODP-01",
      portCount: 8,
    });
  });
});

describe("useAssignOntToOdp", () => {
  beforeEach(() => vi.clearAllMocks());

  it("PUTs the restored /onts/:id/odp route, not a dead distribution endpoint", async () => {
    vi.mocked(apiClient.put).mockResolvedValue({ data: {} } as never);
    const { result } = renderHook(() => useAssignOntToOdp(), {
      wrapper: wrapper(),
    });

    await result.current.mutateAsync({
      ontId: "ont-1",
      odpId: "odp-1",
      port: 3,
    });

    expect(apiClient.put).toHaveBeenCalledWith("/api/v1/onts/ont-1/odp", {
      odpId: "odp-1",
      port: 3,
    });
  });
});

describe("useUnassignOntFromOdp", () => {
  beforeEach(() => vi.clearAllMocks());

  it("DELETEs the restored /onts/:id/odp route", async () => {
    vi.mocked(apiClient.delete).mockResolvedValue({ data: {} } as never);
    const { result } = renderHook(() => useUnassignOntFromOdp(), {
      wrapper: wrapper(),
    });

    await result.current.mutateAsync("ont-1");

    expect(apiClient.delete).toHaveBeenCalledWith("/api/v1/onts/ont-1/odp");
  });
});
