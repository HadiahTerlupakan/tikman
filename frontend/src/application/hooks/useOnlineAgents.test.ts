import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
const unsubscribePresence = vi.fn();
const unsubscribeConnection = vi.fn();
let emit: (ids: string[]) => void = () => {};
let emitConnected: (connected: boolean) => void = () => {};
let signIn = Promise.resolve();

vi.mock("@/infrastructure/firebase/presence", () => ({
  watchPresence: async (onChange: (ids: string[]) => void) => {
    emit = onChange;
    await signIn;
    return unsubscribePresence;
  },
  watchConnection: (onChange: (connected: boolean) => void) => {
    emitConnected = onChange;
    return unsubscribeConnection;
  },
  claimPresence: async () => async () => {},
}));

import { useOnlineAgents } from "./useOnlineAgents";

/** Renders the hook and lets the subscription settle, so a test that drives it
 * is driving this render's own callback. */
async function mount() {
  const rendered = renderHook(() => useOnlineAgents());
  await act(async () => {});
  return rendered;
}

describe("useOnlineAgents", () => {
  beforeEach(() => {
    unsubscribePresence.mockClear();
    unsubscribeConnection.mockClear();
    signIn = Promise.resolve();
  });

  it("hands on whatever the subscription reports", async () => {
    const { result } = await mount();

    await act(async () => emit(["u-rina", "u-budi"]));

    expect(result.current.data).toEqual(["u-rina", "u-budi"]);
  });

  // A subscription left open outlives the page and keeps a socket per visit.
  it("unsubscribes when the page goes away", async () => {
    const { unmount } = await mount();

    unmount();

    expect(unsubscribePresence).toHaveBeenCalledOnce();
    expect(unsubscribeConnection).toHaveBeenCalledOnce();
  });

  // The subscription is attached behind an await for sign-in, so an unmount can
  // land first. Dropped, the listener would outlive the page it belongs to.
  it("unsubscribes a subscription that arrives after the unmount", async () => {
    let finishSignIn: () => void = () => {};
    signIn = new Promise((resolve) => {
      finishSignIn = resolve;
    });

    const { unmount } = renderHook(() => useOnlineAgents());
    unmount();
    await act(async () => finishSignIn());

    expect(unsubscribePresence).toHaveBeenCalledOnce();
  });

  // A frozen list that looks live is worse than one that admits it is stale.
  it("reports the connection separately from the set", async () => {
    const { result } = await mount();

    await act(async () => emitConnected(false));

    expect(result.current.connected).toBe(false);
  });
});
