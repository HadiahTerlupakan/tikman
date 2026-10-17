import { useState } from "react";
import { Button, Dropdown, Modal, Popover, type MenuProps } from "antd";
import { DownOutlined, SmileOutlined } from "@ant-design/icons";
import type { WaGroupMessage } from "@/domain/entities";
import {
  groupMessageActions,
  type GroupMessageAction,
  type GroupMessageActionContext,
  type GroupMessageHandlers,
} from "./groupMessageActions";
import { QUICK_REACTIONS, reactionToSend } from "./reactionChoice";

const labels: Record<Exclude<GroupMessageAction, "react">, string> = {
  reply: "Balas",
  forward: "Teruskan",
  star: "Bintangi",
  info: "Info pesan",
  edit: "Edit",
  revoke: "Hapus untuk semua",
  replyPrivate: "Balas pribadi",
};

function ReactionRow({ onPick }: { onPick: (emoji: string) => void }) {
  return (
    <div style={{ display: "flex", gap: 2 }}>
      {QUICK_REACTIONS.map((emoji) => (
        <Button
          key={emoji}
          type="text"
          aria-label={`Reaksi ${emoji}`}
          onClick={() => onPick(emoji)}
          style={{ fontSize: 20, padding: "0 6px" }}
        >
          {emoji}
        </Button>
      ))}
    </div>
  );
}

function confirmRevoke(onOk: () => void) {
  Modal.confirm({
    title: "Hapus pesan ini untuk semua anggota?",
    content:
      "Pesan diganti dengan keterangan bahwa pesan telah dihapus, di HP setiap anggota.",
    okText: "Hapus untuk semua",
    okButtonProps: { danger: true },
    cancelText: "Batal",
    onOk,
  });
}

/**
 * The hover controls on a bubble: a smiley for WhatsApp's quick reactions and
 * a chevron for everything else. Hidden by opacity rather than removed, so a
 * bubble does not shift as the pointer crosses the thread, and shown on focus
 * as well as hover so the keyboard reaches them.
 */
export function GroupMessageMenu({
  message,
  context,
  handlers,
  visible,
}: {
  message: WaGroupMessage;
  context: GroupMessageActionContext;
  handlers: GroupMessageHandlers;
  visible: boolean;
}) {
  const [focused, setFocused] = useState(false);
  const [picking, setPicking] = useState(false);
  const actions = groupMessageActions(message, context);
  if (actions.length === 0) return null;

  const react = (emoji: string) => {
    setPicking(false);
    handlers.onReact(message, reactionToSend(message.reactions ?? [], emoji));
  };
  const run: Record<Exclude<GroupMessageAction, "react">, () => void> = {
    reply: () => handlers.onReply(message),
    forward: () => handlers.onForward(message),
    star: () => handlers.onStar(message),
    info: () => handlers.onInfo(message),
    edit: () => handlers.onEdit(message),
    revoke: () => confirmRevoke(() => handlers.onRevoke(message)),
    replyPrivate: () => handlers.onReplyPrivate(message),
  };

  const items: MenuProps["items"] = actions.map((action) =>
    action === "react"
      ? {
          key: "react",
          label: "Reaksi",
          children: QUICK_REACTIONS.map((emoji) => ({
            key: `react:${emoji}`,
            label: emoji,
          })),
        }
      : {
          key: action,
          label:
            action === "star" && message.starred
              ? "Hapus bintang"
              : labels[action],
          danger: action === "revoke",
        },
  );
  const onClick: MenuProps["onClick"] = ({ key }) => {
    if (key.startsWith("react:")) react(key.slice("react:".length));
    else run[key as Exclude<GroupMessageAction, "react">]();
  };

  const style = {
    opacity: visible || focused || picking ? 1 : 0,
    transition: "opacity 120ms",
  };
  return (
    <div
      style={{ display: "flex", flexShrink: 0 }}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
    >
      {actions.includes("react") && (
        <Popover
          open={picking}
          onOpenChange={setPicking}
          trigger="click"
          content={<ReactionRow onPick={react} />}
        >
          <Button
            type="text"
            size="small"
            icon={<SmileOutlined />}
            aria-label="Beri reaksi"
            title="Reaksi"
            style={style}
          />
        </Popover>
      )}
      <Dropdown menu={{ items, onClick }} trigger={["click"]}>
        <Button
          type="text"
          size="small"
          icon={<DownOutlined />}
          aria-label="Menu pesan"
          title="Menu pesan"
          style={style}
        />
      </Dropdown>
    </div>
  );
}
