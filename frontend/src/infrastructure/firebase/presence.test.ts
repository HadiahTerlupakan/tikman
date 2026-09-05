import { describe, expect, it, vi } from "vitest";
import {
  attachAfterSignIn,
  claimWhileConnected,
  writePresence,
  type PresenceRef,
} from "./presence";

/** A stand-in for the RTDB node that records what order it was called in. The
 * Firebase SDK is deliberately never loaded here: what is worth testing is the
 * ordering, and the SDK would only need a live database to answer. */
function fakeNode(order: string[]): PresenceRef {
  return {
    onDisconnect: () => ({
      remove: async () => {
        order.push("armed");
      },
    }),
    set: async () => {
      order.push("written");
    },
    remove: async () => {
      order.push("removed");
    },
  };
}

const ignoreFailure = () => {};

describe("writePresence", () => {
  // A socket that dies between the two calls must leave nothing behind. If the
  // value is written first, that window leaves a ghost online until the
  // mirror's next pass.
  it("arms the disconnect handler before writing the value", async () => {
    const order: string[] = [];

    await writePresence(fakeNode(order));

    expect(order).toEqual(["armed", "written"]);
  });
});

describe("claimWhileConnected", () => {
  // Leaving the page is a normal exit and must clear the node itself, rather
  // than relying on the socket closing a moment later.
  it("removes the node when released", async () => {
    const order: string[] = [];
    const stop = vi.fn();

    const release = claimWhileConnected(
      fakeNode(order),
      (onChange) => {
        onChange(true);
        return stop;
      },
      ignoreFailure,
    );
    await vi.waitFor(() => expect(order).toEqual(["armed", "written"]));
    await release();

    expect(order).toEqual(["armed", "written", "removed"]);
    expect(stop).toHaveBeenCalledOnce();
  });

  // The regression this replaced the SSE heartbeat's self-healing with. The
  // server runs the onDisconnect when the socket drops, and the SDK replays
  // neither a completed set nor an onDisconnect armed while connected — so a
  // claim made once on mount is gone for good after one wifi flap, and the
  // agent silently stops receiving work.
  it("arms and writes again on every reconnect", async () => {
    const order: string[] = [];
    let emit: (connected: boolean) => void = () => {};

    claimWhileConnected(
      fakeNode(order),
      (onChange) => {
        emit = onChange;
        return () => {};
      },
      ignoreFailure,
    );

    emit(true);
    await vi.waitFor(() => expect(order).toEqual(["armed", "written"]));
    emit(false);
    emit(true);
    await vi.waitFor(() =>
      expect(order).toEqual(["armed", "written", "armed", "written"]),
    );
  });

  it("stops claiming once released", async () => {
    const order: string[] = [];
    let emit: (connected: boolean) => void = () => {};

    const release = claimWhileConnected(
      fakeNode(order),
      (onChange) => {
        emit = onChange;
        return () => {};
      },
      ignoreFailure,
    );
    await release();
    emit(true);
    await vi.waitFor(() => expect(order).toEqual(["removed"]));
  });

  // A remount mid-write is ordinary — a CS clicking away from the inbox while
  // the first claim is in flight. Without this the write lands after release's
  // remove and the node outlives the page.
  it("removes a claim that lands after the release", async () => {
    const order: string[] = [];
    let letWriteFinish: () => void = () => {};
    const node: PresenceRef = {
      ...fakeNode(order),
      set: () =>
        new Promise((resolve) => {
          letWriteFinish = () => {
            order.push("written");
            resolve(undefined);
          };
        }),
    };

    const release = claimWhileConnected(
      node,
      (onChange) => {
        onChange(true);
        return () => {};
      },
      ignoreFailure,
    );
    await release();
    letWriteFinish();

    await vi.waitFor(() =>
      expect(order).toEqual(["armed", "removed", "written", "removed"]),
    );
  });

  // A refused write is the one failure with no symptom: the node simply is not
  // there, and the mirror drops the agent from the rotation within fifteen
  // seconds without anything having been logged.
  it("reports a refused write rather than swallowing it", async () => {
    const onFailed = vi.fn();
    const failure = new Error("permission_denied");
    const node: PresenceRef = {
      onDisconnect: () => ({ remove: async () => {} }),
      set: async () => {
        throw failure;
      },
      remove: async () => {},
    };

    claimWhileConnected(
      node,
      (onChange) => {
        onChange(true);
        return () => {};
      },
      onFailed,
    );

    await vi.waitFor(() => expect(onFailed).toHaveBeenCalledWith(failure));
  });
});

describe("attachAfterSignIn", () => {
  // The listen the rules deny is not retried: the SDK drops the registration
  // and never re-sends it when the token arrives, so attaching one moment early
  // shows an empty panel for the whole visit with nothing logged.
  it("does not attach the listener until sign-in resolves", async () => {
    const order: string[] = [];
    let finishSignIn: () => void = () => {};
    const signedIn = new Promise<void>((resolve) => {
      finishSignIn = () => {
        order.push("signed-in");
        resolve();
      };
    });

    const attached = attachAfterSignIn(
      () => signedIn,
      () => {
        order.push("attached");
        return () => {};
      },
    );
    await Promise.resolve();
    expect(order).toEqual([]);

    finishSignIn();
    await attached;

    expect(order).toEqual(["signed-in", "attached"]);
  });
});
