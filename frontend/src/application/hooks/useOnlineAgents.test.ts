import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import type {
  ClaimReporter,
  PresenceLink,
} from "@/infrastructure/firebase/presence";

const unsubscribePresence = vi.fn();
const unsubscribeConnection = vi.fn();
const releaseClaim = vi.fn(async () => {});
let emit: (ids: string[]) => void = () => {};
let emitLink: (link: PresenceLink) => void = () => {};
let reporter: ClaimReporter = { onClaimed: () => {}, onFailed: () => {} };
let signIn = Promise.resolve();

vi.mock("@/infrastructure/firebase/presence", () => ({
  watchPresence: async (onChange: (ids: string[]) => void) => {
    emit = onChange;
    await signIn;
    return unsubscribePresence;
  },
  watchConnection: (onChange: (link: PresenceLink) => void) => {
    emitLink = onChange;
    return unsubscribeConnection;
  },
  claimPresence: async (report: ClaimReporter) => {
    reporter = report;
    return releaseClaim;
  },
}));

import { useOnlineAgents } from "./useOnlineAgents";

/** Renders the hook and lets both subscriptions and the claim settle, so a test
 * that drives them is driving this render's own callbacks. */
async function mount() {
  const rendered = renderHook(() => useOnlineAgents());
  await act(async () => {});
  return rendered;
}

describe("useOnlineAgents", () => {
  beforeEach(() => {
    unsubscribePresence.mockClear();
    unsubscribeConnection.mockClear();
    releaseClaim.mockClear();
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
    expect(releaseClaim).toHaveBeenCalledOnce();
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

  // A deployment with no Firebase project reports no link at all, and a healthy
  // one reports false until the socket comes up. Neither is a disconnection an
  // agent can act on, and a banner standing under an empty panel from the first
  // paint is how the real warning stops being read.
  it("says nothing until a connection has existed and gone away", async () => {
    const { result } = await mount();
    expect(result.current.status).toBe("ok");

    await act(async () => emitLink("disconnected"));
    expect(result.current.status).toBe("ok");

    await act(async () => emitLink("connected"));
    expect(result.current.status).toBe("ok");

    await act(async () => emitLink("disconnected"));
    expect(result.current.status).toBe("stale");

    await act(async () => emitLink("connected"));
    expect(result.current.status).toBe("ok");
  });

  it("reports an unconfigured project as nothing to say", async () => {
    const { result } = await mount();

    await act(async () => emitLink("unconfigured"));

    expect(result.current.status).toBe("ok");
  });

  // The one an agent cannot see for themselves: the panel still lists the team,
  // but no new chat will ever be handed to this browser.
  it("reports a failed claim, and clears it when a reconnect succeeds", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = await mount();

    await act(async () => reporter.onFailed(new Error("permission_denied")));
    expect(result.current.status).toBe("unclaimed");

    await act(async () => reporter.onClaimed());
    expect(result.current.status).toBe("ok");

    warn.mockRestore();
  });

  // Both are true at once when the network drops, and only one of them costs
  // the agent their share of the queue.
  it("prefers the failed claim over the stale list", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = await mount();

    await act(async () => emitLink("connected"));
    await act(async () => emitLink("disconnected"));
    await act(async () => reporter.onFailed(new Error("network")));

    expect(result.current.status).toBe("unclaimed");

    warn.mockRestore();
  });
});
