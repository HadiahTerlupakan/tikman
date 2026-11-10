import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useDeleteWaGroup } from "../hooks/useWaGroupActions";
import { waGroupKeys } from "../hooks/waGroupKeys";

const remove = vi.fn();
const error = vi.fn();

vi.mock("@/infrastructure/repositories", () => ({
  WaGroupRepository: class {
    remove(id: string) {
      return remove(id);
    }
  },
}));

vi.mock("antd", () => ({ message: { error: (text: string) => error(text) } }));

function mount() {
  const client = new QueryClient();
  client.setQueryData(waGroupKeys.detail("g1"), { group: { id: "g1" } });
  client.setQueryData(waGroupKeys.list(), [{ id: "g1" }]);
  const { result } = renderHook(() => useDeleteWaGroup(), {
    wrapper: ({ children }) =>
      React.createElement(QueryClientProvider, { client }, children),
  });
  return { client, result };
}

describe("useDeleteWaGroup", () => {
  // Refetching a deleted group's detail would only 404; the list has to
  // drop it at once rather than on the next stream event.
  it("forgets the deleted group and refetches the list", async () => {
    remove.mockResolvedValue(undefined);
    const { client, result } = mount();

    await act(() => result.current.mutateAsync("g1"));

    expect(remove).toHaveBeenCalledWith("g1");
    expect(client.getQueryData(waGroupKeys.detail("g1"))).toBeUndefined();
    expect(client.getQueryState(waGroupKeys.list())?.isInvalidated).toBe(true);
  });

  it("shows the server's reason when the delete is refused", async () => {
    remove.mockRejectedValue(
      new Error("Grup hanya bisa dihapus setelah nomor ini keluar dari grup"),
    );
    const { client, result } = mount();

    await act(() => result.current.mutateAsync("g1").catch(() => {}));

    expect(error).toHaveBeenCalledWith(
      "Grup hanya bisa dihapus setelah nomor ini keluar dari grup",
    );
    expect(client.getQueryData(waGroupKeys.detail("g1"))).toBeDefined();
  });
});
