import { describe, expect, it } from "vitest";
import { filterFor, isInboxView } from "../InboxFilterBar";

describe("filterFor", () => {
  // The backend checks mine, then awaiting-reply, then closed, and the first one
  // set wins — so exactly one may ever be true.
  it("sets exactly one view flag", () => {
    expect(filterFor("semua", "")).toEqual({});
    expect(filterFor("milik-saya", "")).toEqual({ mine: true });
    expect(filterFor("belum-dibalas", "")).toEqual({ awaitingReply: true });
    expect(filterFor("selesai", "")).toEqual({ closed: true });
  });

  it("carries the search term into every view", () => {
    expect(filterFor("milik-saya", "budi")).toEqual({
      mine: true,
      search: "budi",
    });
  });

  // An empty search must be absent rather than "", or the query key changes
  // on every keystroke that clears the box and refetches for nothing.
  it("leaves an empty search out of the filter", () => {
    expect(filterFor("semua", "")).not.toHaveProperty("search");
  });
});

describe("isInboxView", () => {
  // The navbar bell links to ?view=belum-dibalas so the count it shows and the
  // list you land on are the same set. Anything else off the URL — a typo, or a
  // view that was renamed since someone bookmarked it — has to fall back rather
  // than leave the segmented control on a value it cannot render.
  it("accepts the four real views and nothing else", () => {
    expect(isInboxView("semua")).toBe(true);
    expect(isInboxView("milik-saya")).toBe(true);
    expect(isInboxView("belum-dibalas")).toBe(true);
    expect(isInboxView("selesai")).toBe(true);

    expect(isInboxView("belum_dibalas")).toBe(false);
    expect(isInboxView("")).toBe(false);
    expect(isInboxView(null)).toBe(false);
  });
});
