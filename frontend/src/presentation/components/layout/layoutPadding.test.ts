import { describe, expect, it } from "vitest";
import { layoutPadding } from "./layoutPadding";

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
