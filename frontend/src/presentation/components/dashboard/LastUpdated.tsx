import { useEffect, useState } from "react";
import { colors } from "@/shared/theme";
import { formatAge } from "@/presentation/pages/dashboardStats";

const TICK_MS = 5_000;

interface LastUpdatedProps {
  /** React Query's dataUpdatedAt. Undefined until the first response lands. */
  updatedAt: number | undefined;
  isFetching?: boolean;
}

/**
 * How old the figures on this page are. It owns its own timer so the label can
 * keep counting between polls without re-rendering the tables above it.
 */
export function LastUpdated({ updatedAt, isFetching }: LastUpdatedProps) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), TICK_MS);
    return () => window.clearInterval(timer);
  }, []);

  if (!updatedAt) {
    return null;
  }

  return (
    <span style={{ color: colors.textMuted, fontSize: 12 }}>
      {isFetching ? "Refreshing…" : `Updated ${formatAge(now - updatedAt)}`}
    </span>
  );
}
