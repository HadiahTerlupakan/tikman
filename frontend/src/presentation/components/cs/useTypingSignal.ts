import { useEffect, useRef } from "react";

/** How often the "typing…" line is refreshed while a CS keeps writing. A CS
 * types faster than WhatsApp needs to hear about it, and the line looks
 * identical whether it was refreshed once a keystroke or once every few
 * seconds. */
const TYPING_REFRESH_MS = 5_000;

/** How long a CS can stop mid-word before the line comes down. Shorter than
 * the stream's own expiry, so the other side stops seeing it because the CS
 * stopped rather than because a timer ran out. */
const TYPING_IDLE_MS = 3_000;

/**
 * Raises and clears the "sedang mengetik" line the other side of a chat sees
 * — a customer's thread or a group — throttled, dropped after a pause, and
 * cleared on the chat that was left when the CS switches mid-word. Answers
 * the function to call with whether the box holds anything.
 */
export function useTypingSignal(
  targetId: string,
  onTypingChange: (targetId: string, typing: boolean) => void,
) {
  // Which chat currently sees a line, null when none does.
  const typingFor = useRef<string | null>(null);
  const lastSignalAt = useRef(0);
  const idle = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // The latest callback, so the unmount cleanup below does not have to hold a
  // stale one — the chat it names travels with the call, not with the
  // closure.
  const notify = useRef(onTypingChange);
  notify.current = onTypingChange;

  const signalTyping = (typing: boolean) => {
    clearTimeout(idle.current);
    if (typing) {
      idle.current = setTimeout(() => signalTyping(false), TYPING_IDLE_MS);
      const unchanged = typingFor.current === targetId;
      if (unchanged && Date.now() - lastSignalAt.current < TYPING_REFRESH_MS) {
        return;
      }
      typingFor.current = targetId;
      lastSignalAt.current = Date.now();
      notify.current(targetId, true);
      return;
    }
    const showing = typingFor.current;
    if (!showing) return;
    typingFor.current = null;
    notify.current(showing, false);
  };

  useEffect(() => {
    // Switching chats mid-word clears the line on the chat that was left.
    // typingFor still names it here: this runs after the render that brought
    // the new chat in, and nothing has signalled on it yet.
    if (typingFor.current && typingFor.current !== targetId) {
      const left = typingFor.current;
      typingFor.current = null;
      clearTimeout(idle.current);
      notify.current(left, false);
    }
  }, [targetId]);

  useEffect(
    () => () => {
      clearTimeout(idle.current);
      if (typingFor.current) {
        notify.current(typingFor.current, false);
      }
    },
    [],
  );

  return signalTyping;
}
