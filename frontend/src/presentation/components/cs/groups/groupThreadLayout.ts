import type { WaGroupMessage } from "@/domain/entities";

/** The element id a reply or a starred entry scrolls to. */
export function messageAnchor(messageId: string): string {
  return `wa-group-message-${messageId}`;
}

/** Unlike a customer thread, a group has many people on the other side, so a
 * bubble names its sender — once per run, the way WhatsApp does, rather than
 * repeating the same name on every line somebody sends in a row. A notice or
 * a new day breaks the run. */
export function startsRun(
  message: WaGroupMessage,
  previous?: WaGroupMessage,
): boolean {
  if (message.direction === "out" || message.kind === "system") return false;
  if (!previous || previous.kind === "system") return true;
  if (startsDay(message, previous)) return true;
  return previous.senderJid !== message.senderJid;
}

function dayKey(iso: string): string {
  return new Date(iso).toDateString();
}

export function startsDay(
  message: WaGroupMessage,
  previous?: WaGroupMessage,
): boolean {
  return (
    !previous || dayKey(previous.waTimestamp) !== dayKey(message.waTimestamp)
  );
}

/** The pill between days: "Hari ini", "Kemarin", else the date. */
export function dayLabel(iso: string, now = new Date()): string {
  const day = new Date(iso);
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (day.toDateString() === now.toDateString()) return "Hari ini";
  if (day.toDateString() === yesterday.toDateString()) return "Kemarin";
  return day.toLocaleDateString("id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
