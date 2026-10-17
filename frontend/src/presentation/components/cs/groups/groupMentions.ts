import type { WaGroupParticipant } from "@/domain/entities";
import { participantLabel } from "./groupSummary";

/** What can be picked after "@": every member but our own number. */
export function mentionOptions(participants: WaGroupParticipant[]) {
  return participants
    .filter((p) => !p.isSelf)
    .map((p) => ({
      key: p.jid,
      value: participantLabel(p),
      label: participantLabel(p),
    }));
}

/**
 * Turns "@Pak Joko" as typed into what WhatsApp carries: the member's address
 * in the text, and their JID beside it. The text alone highlights nothing and
 * notifies nobody. Longer names go first so "@Budi Santoso" is not taken for
 * "@Budi" followed by a word.
 */
export function toWhatsAppMentions(
  text: string,
  participants: WaGroupParticipant[],
): { body: string; mentions: string[] } {
  const members = participants
    .filter((p) => !p.isSelf)
    .map((p) => ({ jid: p.jid, label: participantLabel(p) }))
    .sort((a, b) => b.label.length - a.label.length);

  let body = text;
  const mentions: string[] = [];
  for (const { jid, label } of members) {
    const tag = `@${label}`;
    if (!body.includes(tag)) continue;
    body = body.split(tag).join(`@${jid.split("@")[0]}`);
    mentions.push(jid);
  }
  return { body, mentions };
}
