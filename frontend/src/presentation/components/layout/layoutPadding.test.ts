import { describe, expect, it } from "vitest";
import { fullHeightPage, layoutPadding } from "./layoutPadding";

describe("layoutPadding", () => {
  it("leaves the desktop gutters as ProLayout sets them", () => {
    // undefined, not 40: restating the library's own number here would drift
    // silently the day it changes.
    expect(layoutPadding({ sm: true, md: true, lg: true })).toEqual({
      contentInline: undefined,
      page: 24,
    });
  });

  it("narrows the gutters on a phone", () => {
    // Measured at 375px: ProLayout's 40 plus our 24 left the content 247px
    // wide. A third of the screen was margin, on every page.
    const pad = layoutPadding({ xs: true });

    expect(pad.contentInline).toBeLessThan(24);
    expect(pad.page).toBeLessThan(24);
  });

  it("reads the first render as desktop, not as a phone", () => {
    // Grid.useBreakpoint returns {} until Ant has measured. Treating that as a
    // phone would flash the narrow layout on every desktop load.
    expect(layoutPadding({})).toEqual({ contentInline: undefined, page: 24 });
  });
});

describe("fullHeightPage", () => {
  // The CS inbox used to hardcode "calc(100vh - 96px)", which counted the
  // header and one padding instead of both. Eight pixels of overflow was
  // enough to make the whole document scroll under a three-column layout that
  // is supposed to fit exactly. The credit footer is in the same reckoning:
  // it sits below the content, so a page sized to the viewport gives up its
  // height too.
  it("subtracts the header, the padding at both ends, and the footer", () => {
    expect(fullHeightPage({ sm: true, md: true, lg: true })).toBe(
      "calc(100vh - 152px)",
    );
  });

  // The gutters differ on a phone, and a page that fits the desktop while
  // overflowing the phone is the same bug in a narrower window.
  it("follows the gutters the screen size gets", () => {
    expect(fullHeightPage({ xs: true })).toBe("calc(100vh - 128px)");
  });
});
