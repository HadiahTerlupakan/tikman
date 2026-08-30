import { useEffect, useState } from "react";

/**
 * Holds a value back until it has stopped changing.
 *
 * Search moved to the server, where every keystroke would otherwise be its own
 * query against the whole ONT table. Typing a twelve-character serial should
 * cost one request, not twelve.
 */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return settled;
}
