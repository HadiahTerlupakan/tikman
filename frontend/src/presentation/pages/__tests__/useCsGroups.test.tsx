import { act, renderHook } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NotFoundError } from "@/infrastructure/http/errorMapper";
import type { WaGroup } from "@/domain/entities";
import { useCsGroups } from "../useCsGroups";

const state = vi.hoisted(() => ({
  groups: [] as WaGroup[],
  detail: {} as { data?: { group: WaGroup }; error?: unknown },
  deleteCallbacks: [] as { onSuccess?: () => void }[],
  mutation: () => ({ mutate: () => undefined, mutateAsync: async () => {} }),
}));

vi.mock("@/application/hooks", () => ({
  useCreateWaGroup: state.mutation,
  useDeleteWaGroup: () => ({
    mutate: (_id: string, callbacks: { onSuccess?: () => void }) =>
      state.deleteCallbacks.push(callbacks),
    isPending: false,
  }),
  useRefreshWaGroups: state.mutation,
  useSendWaGroupMedia: state.mutation,
  useSendWaGroupMessage: state.mutation,
  useWaAccounts: () => ({ data: [] }),
  useWaGroupAction: state.mutation,
  useWaGroupActionTracker: () => vi.fn(),
  useWaGroupDetail: () => state.detail,
  useWaGroupHistory: () => ({}),
  useWaGroups: () => ({ data: state.groups }),
}));

vi.mock("../useGroupMessageActions", () => ({
  useGroupMessageActions: () => ({}),
}));

const group = (id: string) => ({ id, name: `Grup ${id}` }) as WaGroup;

function renderAt(url: string) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[url]}>{children}</MemoryRouter>
  );
  return renderHook(
    () => ({ groups: useCsGroups(), search: useLocation().search }),
    { wrapper },
  );
}

describe("useCsGroups", () => {
  beforeEach(() => {
    state.groups = [];
    state.detail = {};
    state.deleteCallbacks = [];
  });

  it("drops a selected group whose detail answers 404 despite stale data", () => {
    state.detail = {
      data: { group: group("g1") },
      error: new NotFoundError("group"),
    };

    const { result } = renderAt("/cs-groups?group=g1&panel=info");

    expect(result.current.groups.selected).toBeUndefined();
    expect(result.current.search).toBe("");
  });

  it("keeps the stale detail when the refetch failed for another reason", () => {
    state.detail = { data: { group: group("g1") }, error: new Error("net") };

    const { result } = renderAt("/cs-groups?group=g1");

    expect(result.current.groups.selected?.id).toBe("g1");
    expect(result.current.search).toBe("?group=g1");
  });

  it("keeps a group opened while the delete of another was in flight", () => {
    state.groups = [group("g1"), group("g2")];
    const { result } = renderAt("/cs-groups?group=g1");

    act(() => result.current.groups.deleteGroup("g1"));
    act(() => result.current.groups.handleSelect("g2"));
    act(() => state.deleteCallbacks[0].onSuccess?.());

    expect(result.current.groups.selected?.id).toBe("g2");
    expect(result.current.search).toBe("?group=g2");
  });

  it("clears the selection when the deleted group is still open", () => {
    state.groups = [group("g1")];
    const { result } = renderAt("/cs-groups?group=g1");

    act(() => result.current.groups.deleteGroup("g1"));
    act(() => state.deleteCallbacks[0].onSuccess?.());

    expect(result.current.search).toBe("");
  });
});
