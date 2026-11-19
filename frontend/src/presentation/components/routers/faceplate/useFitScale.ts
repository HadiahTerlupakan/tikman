import { type RefObject, useLayoutEffect, useState } from "react";

/** The scale that fits a chassis of `width` into the element, kept in step as it resizes. */
export function useFitScale(ref: RefObject<HTMLElement | null>, width: number) {
  const [scale, setScale] = useState(1);
  useLayoutEffect(() => {
    const el = ref.current;
    // The rack's shared width is 0 until a unit reports its board.
    if (!el || width <= 0) return;
    const fit = () => setScale(el.clientWidth / width);
    fit();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(fit);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref, width]);
  return scale;
}

// Below this the labels' text gets too small to read, so a narrow screen
// scrolls the stage sideways instead of shrinking it further.
export const MIN_STAGE_SCALE = 0.6;

/** The detail stage's scale: shrunk to fit, never enlarged, and never below the readable floor. */
export function stageScale(fit: number): number {
  // An element not laid out yet reports no width, which is no reason to shrink.
  if (!(fit > 0)) return 1;
  return Math.min(1, Math.max(MIN_STAGE_SCALE, fit));
}
