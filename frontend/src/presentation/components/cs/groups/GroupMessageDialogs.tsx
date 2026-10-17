import type { WaGroup, WaGroupMessage } from "@/domain/entities";
import { ForwardModal } from "./ForwardModal";
import { GroupFieldModal } from "./GroupFieldModal";
import { MessageInfoModal } from "./MessageInfoModal";

/** Which dialog a bubble's menu opened, and on which message. */
export interface GroupMessageDialogState {
  kind: "forward" | "edit" | "info";
  message: WaGroupMessage;
}

/** The dialogs behind the bubble menu: edit, forward and Info pesan. */
export function GroupMessageDialogs({
  dialog,
  groups,
  forwarding,
  onEdit,
  onForward,
  onClose,
}: {
  dialog?: GroupMessageDialogState;
  /** Where a message can be forwarded to. */
  groups: WaGroup[];
  forwarding: boolean;
  onEdit: (message: WaGroupMessage, body: string) => void;
  onForward: (message: WaGroupMessage, groupIds: string[]) => void;
  onClose: () => void;
}) {
  const target = dialog?.message;
  return (
    <>
      <GroupFieldModal
        open={dialog?.kind === "edit"}
        title="Edit pesan"
        hint="Pesan yang diedit ditandai Diedit di HP setiap anggota. Hanya bisa dalam 15 menit setelah dikirim."
        initialValue={target?.body ?? ""}
        multiline
        okText="Simpan"
        onSubmit={(body) => target && onEdit(target, body)}
        onClose={onClose}
      />
      <ForwardModal
        open={dialog?.kind === "forward"}
        groups={groups}
        forwarding={forwarding}
        onForward={(ids) => target && onForward(target, ids)}
        onClose={onClose}
      />
      <MessageInfoModal
        message={dialog?.kind === "info" ? target : undefined}
        onClose={onClose}
      />
    </>
  );
}
