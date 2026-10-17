import type { WaGroupAction } from "@/domain/entities";

const doneText: Record<WaGroupAction["action"], string> = {
  create: "Grup dibuat",
  rename: "Nama grup diubah",
  set_topic: "Deskripsi grup diubah",
  add: "Anggota ditambahkan",
  remove: "Anggota dikeluarkan",
  promote: "Anggota dijadikan admin",
  demote: "Admin dijadikan anggota biasa",
  set_announce: "Pengaturan pengiriman pesan diubah",
  set_locked: "Pengaturan info grup diubah",
  invite_link: "Tautan undangan siap",
  reset_invite_link: "Tautan undangan baru dibuat",
  leave: "Nomor ini sudah keluar dari grup",
  react: "Reaksi terkirim",
  edit: "Pesan diedit",
  revoke: "Pesan dihapus untuk semua",
  vote: "Pilihan terkirim",
  history: "Riwayat lama diminta dari HP",
};

export interface GroupActionOutcome {
  level: "success" | "warning" | "error";
  text: string;
  /** Set only for the two link actions, whose result is the link itself. */
  inviteLink?: string;
}

/**
 * What to tell an admin once an action has settled. A member action can be
 * done and still have skipped somebody — WhatsApp answers per number — so a
 * non-empty result on one of those is a warning naming who, not a success.
 */
export function describeOutcome(action: WaGroupAction): GroupActionOutcome {
  if (action.status === "failed") {
    return {
      level: "error",
      text: action.failReason || "WhatsApp menolak permintaan ini",
    };
  }
  const isLink =
    action.action === "invite_link" || action.action === "reset_invite_link";
  if (isLink) {
    return {
      level: "success",
      text: doneText[action.action],
      inviteLink: action.result || undefined,
    };
  }
  if (action.result) {
    return {
      level: "warning",
      text: `${doneText[action.action]}, dengan catatan: ${action.result}`,
    };
  }
  return { level: "success", text: doneText[action.action] };
}
