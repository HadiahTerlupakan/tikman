import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const unsubscribePresence = vi.fn();
const unsubscribeConnection = vi.fn();
let emit: (ids: string[]) => void = () => {};
let emitConnected: (connected: boolean) => void = () => {};

vi.mock("@/infrastructure/firebase/presence", () => ({
  watchPresence: (onChange: (ids: string[]) => void) => {
    emit = onChange;
    return unsubscribePresence;
  },
  watchConnection: (onChange: (connected: boolean) => void) => {
    emitConnected = onChange;
    return unsubscribeConnection;
  },
  claimPresence: async () => async () => {},
}));

import { useOnlineAgents } from "./useOnlineAgents";

describe("useOnlineAgents", () => {
  beforeEach(() => {
    unsubscribePresence.mockClear();
    unsubscribeConnection.mockClear();
  });

  it("hands on whatever the subscription reports", () => {
    const { result } = renderHook(() => useOnlineAgents());

    act(() => emit(["u-rina", "u-budi"]));

    expect(result.current.data).toEqual(["u-rina", "u-budi"]);
  });

  // A subscription left open outlives the page and keeps a socket per visit.
  it("unsubscribes when the page goes away", () => {
    const { unmount } = renderHook(() => useOnlineAgents());

    unmount();

    expect(unsubscribePresence).toHaveBeenCalledOnce();
    expect(unsubscribeConnection).toHaveBeenCalledOnce();
  });

  // A frozen list that looks live is worse than one that admits it is stale.
  it("reports the connection separately from the set", () => {
    const { result } = renderHook(() => useOnlineAgents());

    act(() => emitConnected(false));

    expect(result.current.connected).toBe(false);
  });
});
