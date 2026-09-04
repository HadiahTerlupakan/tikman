import { describe, expect, it } from "vitest";
import { inboxLayout } from "./inboxLayout";

const phone = { xs: true };
const desktop = { sm: true, md: true, lg: true };

describe("inboxLayout", () => {
  // Three columns need roughly a thousand pixels: the list alone is fixed at
  // 340. On a phone they have to take turns instead.
  it("keeps all three columns on a wide screen", () => {
    expect(inboxLayout(desktop, undefined, null)).toEqual({
      columns: true,
      pane: "list",
    });
  });

  // The URL carries the selection, so a desktop link opened on a desktop must
  // not start navigating panes just because a conversation is named in it.
  it("keeps all three columns even when a conversation is chosen", () => {
    expect(inboxLayout(desktop, "c1", "customer")).toEqual({
      columns: true,
      pane: "list",
    });
  });

  it("shows the list on a phone with nothing chosen", () => {
    expect(inboxLayout(phone, undefined, null)).toEqual({
      columns: false,
      pane: "list",
    });
  });

  it("shows the thread on a phone once a conversation is chosen", () => {
    expect(inboxLayout(phone, "c1", null)).toEqual({
      columns: false,
      pane: "thread",
    });
  });

  it("shows the customer on a phone when the URL asks for it", () => {
    expect(inboxLayout(phone, "c1", "customer")).toEqual({
      columns: false,
      pane: "customer",
    });
  });

  // A shared or half-typed link can name the customer pane without naming a
  // conversation. There is no customer to show, and falling back to the thread
  // would render an empty one — so the list is the only honest answer.
  it("falls back to the list when the customer pane names no conversation", () => {
    expect(inboxLayout(phone, undefined, "customer")).toEqual({
      columns: false,
      pane: "list",
    });
  });

  // Ant hands back an empty object before it has measured anything. Reading
  // that as a phone would flash the single-pane layout on every desktop load,
  // the same reason layoutPadding keys on xs rather than the absence of md.
  it("reads the unmeasured first render as a wide screen", () => {
    expect(inboxLayout({}, "c1", null)).toEqual({
      columns: true,
      pane: "list",
    });
  });
});
