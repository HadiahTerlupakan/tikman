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

/** A sender as a group member would recognise them. WhatsApp often hides the
 * number behind a LID, so the jid is the last resort, not the first. */
export function senderLabel(message: WaGroupMessage): string {
  if (message.direction === "out") return "Anda";
  if (message.senderName) return message.senderName;
  if (message.senderPhone) return `+${message.senderPhone}`;
  return message.senderJid.split("@")[0];
}

export function participantLabel(participant: WaGroupParticipant): string {
  if (participant.isSelf) return "Nomor ini";
  if (participant.name) return participant.name;
  if (participant.phone) return `+${participant.phone}`;
  return participant.jid.split("@")[0];
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
