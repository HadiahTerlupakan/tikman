import { useCallback, useEffect, useLayoutEffect, useRef } from "react";
import type { CsMessage } from "@/domain/entities";

/** A reader this close to the end is still reading the newest message. The
 * slack keeps a fractional overshoot or a trackpad's settle from counting as
 * having scrolled away. */
const PINNED_WITHIN_PX = 80;

/**
 * Keeps a conversation log at its newest message: where a thread opens,
 * whoever wrote last, and where it stays while messages arrive — unless the
 * reader has scrolled up to read history, which a new message must not undo.
 */
export function useStickToBottom(
  conversationId: string | undefined,
  messages: CsMessage[],
  loading: boolean,
) {
  const containerRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const pinned = useRef(true);

  const follow = useCallback(() => {
    const container = containerRef.current;
    if (container && pinned.current) {
      container.scrollTop = container.scrollHeight;
    }
  }, []);

  useLayoutEffect(() => {
    pinned.current = true;
  }, [conversationId]);

  useLayoutEffect(follow, [conversationId, messages, loading, follow]);

  // A photo or link preview takes its height only once it has loaded, after
  // the render that placed it, so size changes are followed too. Re-attached
  // per conversation: the log does not exist until one is open.
  useEffect(() => {
    const content = contentRef.current;
    if (!content || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(follow);
    observer.observe(content);
    return () => observer.disconnect();
  }, [conversationId, follow]);

  const onScroll = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;
    const fromEnd =
      container.scrollHeight - container.scrollTop - container.clientHeight;
    pinned.current = fromEnd <= PINNED_WITHIN_PX;
  }, []);

  /** For the CS's own reply: they want to see it land, wherever they were. */
  const stick = useCallback(() => {
    pinned.current = true;
    follow();
  }, [follow]);

  return { containerRef, contentRef, onScroll, stick };
}
