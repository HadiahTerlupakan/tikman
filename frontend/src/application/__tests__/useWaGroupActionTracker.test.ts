import { describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import type { WaGroupAction } from "@/domain/entities";
import { useWaGroupActionTracker } from "../hooks/useWaGroupActions";
import { invalidateForGroupEvent } from "../hooks/waGroupKeys";

const getAction = vi.fn();

vi.mock("@/infrastructure/repositories", () => ({
  WaGroupRepository: class {
    getAction(id: string) {
      return getAction(id);
    }
  },
}));

function action(status: WaGroupAction["status"]): WaGroupAction {
  return {
    id: "a1",
    waAccountId: "acc1",
    action: "rename",
    status,
    result: "",
    failReason: "",
    createdAt: "2026-10-17T07:00:00Z",
  };
}

describe("useWaGroupActionTracker", () => {
  // The outcome has to reach the admin when the stream says the action
  // finished — and only once, however many times the query refetches.
  it("reports a queued action once the stream says it settled", async () => {
    const client = new QueryClient();
    const onSettled = vi.fn();
    getAction.mockResolvedValue(action("queued"));
    const { result } = renderHook(() => useWaGroupActionTracker(onSettled), {
      wrapper: ({ children }) =>
        React.createElement(QueryClientProvider, { client }, children),
    });

    act(() => result.current(action("queued")));
    await waitFor(() => expect(getAction).toHaveBeenCalledTimes(1));
    expect(onSettled).not.toHaveBeenCalled();

    getAction.mockResolvedValue(action("done"));
    act(() =>
      invalidateForGroupEvent(client, {
        type: "group_action",
        action_id: "a1",
      }),
    );

    await waitFor(() => expect(onSettled).toHaveBeenCalledTimes(1));
    expect(onSettled.mock.calls[0][0].status).toBe("done");

    act(() =>
      invalidateForGroupEvent(client, {
        type: "group_action",
        action_id: "a1",
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(onSettled).toHaveBeenCalledTimes(1);
  });

  it("reports an action that came back already settled without fetching it", () => {
    const client = new QueryClient();
    const onSettled = vi.fn();
    getAction.mockClear();
    const { result } = renderHook(() => useWaGroupActionTracker(onSettled), {
      wrapper: ({ children }) =>
        React.createElement(QueryClientProvider, { client }, children),
    });

    act(() => result.current(action("failed")));

    expect(onSettled).toHaveBeenCalledTimes(1);
    expect(getAction).not.toHaveBeenCalled();
  });
});
