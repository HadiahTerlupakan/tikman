import type { WaGroupParticipant } from "@/domain/entities";
import { participantLabel } from "./groupSummary";

/** "@semua" tags every member but our own number. */
export const MENTION_ALL = "semua";

/** WAGroupService.CheckMentions refuses a message tagging more JIDs. */
export const MAX_MENTIONS = 1024;

// "@semua" followed by the end, a space or punctuation: "@semuanya" is a
// word, not the tag.
const MENTION_ALL_PATTERN = `@${MENTION_ALL}(?![\\p{L}\\p{N}_])`;
const MENTION_ALL_TOKEN = new RegExp(MENTION_ALL_PATTERN, "u");
const MENTION_ALL_TOKENS = new RegExp(MENTION_ALL_PATTERN, "gu");
const MENTION_ALL_AT = new RegExp(MENTION_ALL_PATTERN, "uy");
// A private-use character, which no one types.
const EVERYONE_MARK = "\uE000";

/** Whether the text tags everyone. */
export function mentionsEveryone(text: string): boolean {
  return MENTION_ALL_TOKEN.test(text);
}

/** What can be picked after "@": everyone at once, then every member but our
 * own number. */
export function mentionOptions(participants: WaGroupParticipant[]) {
  const members = participants
    .filter((p) => !p.isSelf)
    .map((p) => ({
      key: p.jid,
      value: participantLabel(p),
      label: participantLabel(p),
    }));
  if (members.length === 0) return members;
  const everyone = {
    key: `@${MENTION_ALL}`,
    value: MENTION_ALL,
    label: `${MENTION_ALL} — tag semua anggota`,
  };
  return [everyone, ...members];
}

/** Why a message cannot go out as it is, or null: the API would refuse it
 * with a bare 400. */
export function mentionLimitError(mentions: string[]): string | null {
  if (mentions.length <= MAX_MENTIONS) return null;
  return `Grup terlalu besar untuk @${MENTION_ALL} (maks ${MAX_MENTIONS} anggota). Sebut anggota satu per satu.`;
}

/**
 * Turns "@Pak Joko" as typed into what WhatsApp carries: the member's address
 * in the text, and their JID beside it. The text alone highlights nothing and
 * notifies nobody. Longer names go first so "@Budi Santoso" is not taken for
 * "@Budi" followed by a word.
 *
 * "@semua" stays as written and lists every member's JID: WhatsApp notifies
 * each listed JID whether or not its address is in the text, and hundreds of
 * numbers in the body would bury the message.
 */
export function toWhatsAppMentions(
  text: string,
  participants: WaGroupParticipant[],
): { body: string; mentions: string[] } {
  const others = participants.filter((p) => !p.isSelf);
  // A member named "semua" is not rewritten: the picker offers "@semua" as
  // everyone, and both read the same in the box, so everyone is what it
  // means. That member is tagged with the rest.
  const tags = others
    .map((p) => ({ jid: p.jid, label: participantLabel(p) }))
    .filter((m) => m.label !== MENTION_ALL)
    .concat({ jid: "", label: MENTION_ALL })
    .sort((a, b) => b.label.length - a.label.length);

  let body = text;
  let everyone = false;
  const mentions = new Set<string>();
  for (const { jid, label } of tags) {
    if (!jid) {
      // Set aside rather than left in place, so a shorter name such as "@s"
      // cannot rewrite the start of it.
      everyone = mentionsEveryone(body);
      body = body.replace(MENTION_ALL_TOKENS, EVERYONE_MARK);
      continue;
    }
    const tag = `@${label}`;
    if (!body.includes(tag)) continue;
    body = body.split(tag).join(`@${jid.split("@")[0]}`);
    mentions.add(jid);
  }
  if (everyone) others.forEach((p) => mentions.add(p.jid));
  body = body.split(EVERYONE_MARK).join(`@${MENTION_ALL}`);
  return { body, mentions: [...mentions] };
}

/** The names splitMentions highlights in a stored message: "@semua" counts
 * only when the message really tagged someone. Tagged rather than
 * mentionNames decides it, because members with no name yet are notified but
 * not named. */
export function highlightedNames(
  body: string,
  mentionNames: string[],
  tagged: boolean,
) {
  if (!tagged || !mentionsEveryone(body)) return mentionNames;
  return [...mentionNames, MENTION_ALL];
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
function tagAt(body: string, tag: string, i: number): boolean {
  if (tag !== `@${MENTION_ALL}`) return body.startsWith(tag, i);
  MENTION_ALL_AT.lastIndex = i;
  return MENTION_ALL_AT.test(body);
}

export function splitMentions(body: string, names: string[]): MentionPart[] {
  const tags = [...new Set(names.filter(Boolean))]
    .map((name) => `@${name}`)
    .sort((a, b) => b.length - a.length);
  const parts: MentionPart[] = [];
  let plain = "";
  let i = 0;
  while (i < body.length) {
    const tag = body[i] === "@" && tags.find((t) => tagAt(body, t, i));
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
