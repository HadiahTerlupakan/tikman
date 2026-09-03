import { describe, expect, it } from "vitest";
import { filterFor } from "../InboxFilterBar";

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
