import { describe, expect, it, vi } from "vitest";
import { writePresence } from "./presence";

describe("writePresence", () => {
  // A socket that dies between the two calls must leave nothing behind. If the
  // value is written first, that window leaves a ghost online until the
  // mirror's next pass.
  it("arms the disconnect handler before writing the value", async () => {
    const order: string[] = [];
    const ref = {
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

    const release = await writePresence(ref);

    expect(order).toEqual(["armed", "written"]);
    await release();
    expect(order).toEqual(["armed", "written", "removed"]);
  });

  // Leaving the page is a normal exit and must clear the node itself, rather
  // than relying on the socket closing a moment later.
  it("removes the node when released", async () => {
    const remove = vi.fn(async () => {});
    const ref = {
      onDisconnect: () => ({ remove: async () => {} }),
      set: async () => {},
      remove,
    };

    const release = await writePresence(ref);
    await release();

    expect(remove).toHaveBeenCalledOnce();
  });
});
