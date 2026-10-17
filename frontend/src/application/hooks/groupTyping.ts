/** How long a member's "sedang mengetik" stays up without a refresh. WhatsApp
 * repeats composing every few seconds while someone writes, so a line that
 * has not been refreshed in this long belongs to someone who stopped. */
export const GROUP_TYPING_TTL_MS = 6_000;

/** The group_typing event as the stream sends it. */
export interface GroupTypingEvent {
  group_id?: string;
  sender_jid?: string;
  sender_name?: string;
  typing?: boolean;
}

/** Who is writing in which group, with the time each line expires, keyed by
 * group id and then by member jid. */
export type GroupTypingDeadlines = Record<
  string,
  Record<string, { name: string; until: number }>
>;

/** Names of the members writing right now, keyed by group id. */
export type GroupTypingStatus = Record<string, string[]>;

export function applyGroupTyping(
  current: GroupTypingDeadlines,
  event: GroupTypingEvent,
  now: number,
): GroupTypingDeadlines {
  const { group_id: groupId, sender_jid: jid } = event;
  if (!groupId || !jid) return current;
  const members = { ...current[groupId] };
  if (event.typing) {
    members[jid] = {
      name: event.sender_name || jid.split("@")[0],
      until: now + GROUP_TYPING_TTL_MS,
    };
  } else {
    if (!(jid in members)) return current;
    delete members[jid];
  }
  return withGroup(current, groupId, members);
}

/** Drops every line past its deadline. Answers current itself when nothing
 * expired, so a sweep that changes nothing does not re-render. */
export function sweepGroupTyping(
  current: GroupTypingDeadlines,
  now: number,
): GroupTypingDeadlines {
  let next = current;
  for (const [groupId, members] of Object.entries(current)) {
    const live = Object.fromEntries(
      Object.entries(members).filter(([, entry]) => entry.until > now),
    );
    if (Object.keys(live).length !== Object.keys(members).length) {
      next = withGroup(next, groupId, live);
    }
  }
  return next;
}

export function groupTypingNames(
  deadlines: GroupTypingDeadlines,
): GroupTypingStatus {
  return Object.fromEntries(
    Object.entries(deadlines).map(([groupId, members]) => [
      groupId,
      Object.values(members).map((m) => m.name),
    ]),
  );
}

function withGroup(
  current: GroupTypingDeadlines,
  groupId: string,
  members: GroupTypingDeadlines[string],
): GroupTypingDeadlines {
  const next = { ...current };
  if (Object.keys(members).length === 0) delete next[groupId];
  else next[groupId] = members;
  return next;
}
