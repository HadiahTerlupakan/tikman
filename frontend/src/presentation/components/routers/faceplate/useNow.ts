import { useEffect, useState } from "react";

/**
 * The current time, refreshed every `intervalMs`. A health reading goes stale
 * by the clock alone, while the router data that would re-render the page may
 * not change at all, so freshness needs a clock of its own.
 */
export function useNow(intervalMs: number): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}
