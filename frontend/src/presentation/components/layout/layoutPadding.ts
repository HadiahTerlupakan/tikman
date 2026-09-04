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

/** The app bar's height, which ProLayout is told to render at this size. */
export const HEADER_HEIGHT = 56;

/**
 * fullHeightPage answers the height a page should take to fill the viewport
 * without the document scrolling.
 *
 * Derived rather than written down: the CS inbox carried "calc(100vh - 96px)",
 * which counted the header and one of the two page paddings. It fitted nothing
 * exactly, and eight pixels of overflow was enough to make a layout designed to
 * fit the screen scroll instead. Change the header height or the gutters and
 * this follows; a literal would not.
 *
 * It does not account for anything the layout renders above the page — the
 * notification-permission banner is the one such thing today — so a page using
 * this is exactly as tall as the viewport only while that banner is absent.
 */
export function fullHeightPage(screens: Screens): string {
  const padding = layoutPadding(screens);
  return `calc(100vh - ${HEADER_HEIGHT + padding.page * 2}px)`;
}
