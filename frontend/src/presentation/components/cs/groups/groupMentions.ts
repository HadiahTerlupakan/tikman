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

export interface MentionPart {
  text: string;
  mention: boolean;
}

/**
 * Splits a body into plain runs and the "@<name>" tags WhatsApp marked as
 * mentions, so only those are highlighted — an "@" someone typed by hand
 * notified nobody and is drawn as text. Longer names are tried first, for
 * the reason toWhatsAppMentions gives.
 */
export function splitMentions(body: string, names: string[]): MentionPart[] {
  const tags = [...new Set(names.filter(Boolean))]
    .map((name) => `@${name}`)
    .sort((a, b) => b.length - a.length);
  const parts: MentionPart[] = [];
  let plain = "";
  let i = 0;
  while (i < body.length) {
    const tag = body[i] === "@" && tags.find((t) => body.startsWith(t, i));
    if (!tag) {
      plain += body[i++];
      continue;
    }
    if (plain) parts.push({ text: plain, mention: false });
    parts.push({ text: tag, mention: true });
    plain = "";
    i += tag.length;
  }
  if (plain) parts.push({ text: plain, mention: false });
  return parts;
}
