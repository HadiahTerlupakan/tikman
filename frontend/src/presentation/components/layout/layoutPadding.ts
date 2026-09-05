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
 * The credit footer's height. AppFooter is given this as a fixed height rather
 * than letting its padding decide, so this number stays true instead of
 * approximating what the text happens to measure.
 */
export const FOOTER_HEIGHT = 48;

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
 * It accounts for the shell's own three numbers and nothing else. ProLayout adds
 * spacing of its own around the content area, and the layout renders the
 * notification-permission banner above the page, so a page using this comes
 * close to the viewport rather than matching it. Measured on 2026-09-04 that
 * was near enough to keep; making it exact means measuring the page's real
 * position at render, or turning the whole shell into a flex chain, and both
 * were considered and declined.
 */
export function fullHeightPage(screens: Screens): string {
  const padding = layoutPadding(screens);
  const chrome = HEADER_HEIGHT + padding.page * 2 + FOOTER_HEIGHT;
  return `calc(100vh - ${chrome}px)`;
}
