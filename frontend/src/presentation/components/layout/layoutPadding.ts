import type { Breakpoint } from "antd";

/** What Ant Design's `Grid.useBreakpoint()` hands back. */
export type Screens = Partial<Record<Breakpoint, boolean>>;

export interface LayoutPadding {
  /** Inline padding for ProLayout's content element; undefined keeps its own. */
  contentInline: number | undefined;
  /** Padding on the wrapper this app puts around every page. */
  page: number;
}

const DESKTOP: LayoutPadding = { contentInline: undefined, page: 24 };
const PHONE: LayoutPadding = { contentInline: 8, page: 12 };

/**
 * layoutPadding decides the gutters around a page's content.
 *
 * On a 375px screen the shell spent 72px a side — ProLayout's 40 plus this
 * app's own 24 — leaving the content 247px of a 375px screen. A third of the
 * phone was empty margin, on every page at once.
 *
 * Keyed on `xs` rather than the absence of `md` so that the first render, where
 * Ant has measured nothing and hands back `{}`, reads as desktop. Assuming a
 * phone there would flash the narrow layout on every desktop load.
 */
export function layoutPadding(screens: Screens): LayoutPadding {
  return screens.xs ? PHONE : DESKTOP;
}
