import type { WaGroup, WaGroupMessage } from "@/domain/entities";

export function waGroup(over: Partial<WaGroup> = {}): WaGroup {
  return {
    id: "g1",
    waAccountId: "acc1",
    jid: "120363000000000001@g.us",
    name: "Teknisi Cariu",
    topic: "",
    ownerJid: "",
    participantCount: 4,
    isAdmin: false,
    announce: false,
    locked: false,
    left: false,
    lastMessagePreview: "",
    lastMessageKind: "",
    lastMessageSender: "",
    unreadCount: 0,
    hasAvatar: false,
    pinned: false,
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
    ...over,
  };
}

export function waGroupMessage(
  over: Partial<WaGroupMessage> = {},
): WaGroupMessage {
  return {
    id: "m1",
    groupId: "g1",
    direction: "in",
    senderJid: "628111@s.whatsapp.net",
    senderPhone: "628111",
    senderName: "Budi",
    senderPushName: "",
    senderHasAvatar: false,
    kind: "text",
    body: "ODP 3 mati",
    status: "delivered",
    mentionNames: [],
    edited: false,
    deleted: false,
    forwarded: false,
    starred: false,
    reactions: [],
    waTimestamp: "2026-10-17T07:05:00Z",
    createdAt: "2026-10-17T07:05:00Z",
    ...over,
  };
}
