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
