import type {
  WaGroup,
  WaGroupMessage,
  WaGroupParticipant,
} from "@/domain/entities";
import { preview } from "../conversationSummary";

/** The line under a group's name in the list: who spoke last, and what. The
 * kind label comes from the inbox's own preview so a photo reads "Foto" in
 * both places. */
export function groupPreview(group: WaGroup): string {
  if (!group.lastMessageKind) return "Belum ada pesan";
  const line = preview({
    kind: group.lastMessageKind,
    body: group.lastMessagePreview,
    // preview reads neither of these; they only complete its type.
    direction: "in",
    at: group.lastMessageAt ?? "",
  });
  const sender =
    group.lastMessageDirection === "out" ? "Anda" : group.lastMessageSender;
  return sender ? `${sender}: ${line}` : line;
}

/**
 * Why nothing can be sent to this group from here, or null when it can. The
 * API refuses both cases with a 409; saying so before a CS types the message
 * is the whole point of asking.
 */
export function sendBlockedReason(group: WaGroup): string | null {
  if (group.left) {
    return "Nomor ini sudah keluar dari grup. Riwayat tetap bisa dibaca, tetapi pesan tidak bisa dikirim.";
  }
  if (group.announce && !group.isAdmin) {
    return "Hanya admin grup yang boleh mengirim pesan, dan nomor ini bukan admin.";
  }
  return null;
}

/** WhatsApp's rule for naming a member: the address-book name when there is
 * one, else the name they gave themselves marked with "~" so nobody takes it
 * for a verified contact, else the number. WhatsApp often hides the number
 * behind a LID, so the jid is the last resort, not the first. */
function displayName(
  name: string,
  pushName: string,
  phone: string,
  jid: string,
): string {
  if (name) return name;
  if (pushName) return `~${pushName}`;
  if (phone) return `+${phone}`;
  return jid.split("@")[0];
}

export function senderLabel(
  message: Pick<
    WaGroupMessage,
    "direction" | "senderName" | "senderPushName" | "senderPhone" | "senderJid"
  >,
): string {
  if (message.direction === "out") return "Anda";
  return displayName(
    message.senderName,
    message.senderPushName ?? "",
    message.senderPhone,
    message.senderJid,
  );
}

export function participantLabel(participant: WaGroupParticipant): string {
  if (participant.isSelf) return "Nomor ini";
  return displayName(
    participant.name,
    participant.pushName ?? "",
    participant.phone,
    participant.jid,
  );
}

/** "Budi sedang mengetik…", or both names when two write at once. Past two
 * WhatsApp stops naming people, and so does this. */
export function typingLine(names: string[]): string | null {
  if (names.length === 0) return null;
  if (names.length === 1) return `${names[0]} sedang mengetik…`;
  if (names.length === 2) return `${names[0]} dan ${names[1]} sedang mengetik…`;
  return `${names.length} orang sedang mengetik…`;
}

/** The line under the group's name in its header: its members by name, the
 * way WhatsApp lists them, or only the count until they are loaded. */
export function memberSummary(
  participants: WaGroupParticipant[],
  count: number,
): string {
  if (participants.length === 0) return `${count} anggota`;
  const others = participants.filter((p) => !p.isSelf).map(participantLabel);
  const self = participants.some((p) => p.isSelf) ? ["Anda"] : [];
  return [...others, ...self].join(", ");
}

/** Splits what an admin pasted into a list of numbers. People paste from a
 * spreadsheet column as readily as from a chat, so commas, semicolons and line
 * breaks all separate — but spaces do not, because "0812 3456 7890" is how a
 * number is often written. Normalising the form is left to the server, which
 * accepts 0812…, +62… and 62… alike. */
export function parseParticipants(text: string): string[] {
  return [
    ...new Set(
      text
        .split(/[\n,;]+/)
        // The grouping spaces and dashes go: they are formatting, not digits.
        .map((part) => part.replace(/[\s-]/g, ""))
        .filter(Boolean),
    ),
  ];
}

/** WhatsApp's own limit on a group name. */
export const GROUP_NAME_MAX = 25;

/** Whether our number may change the name and description: any member may
 * while the group is unlocked, only an admin once it is locked. */
export function mayEditInfo(group: WaGroup): boolean {
  return group.isAdmin || !group.locked;
}
