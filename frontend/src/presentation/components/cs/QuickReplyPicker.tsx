import { useState } from "react";
import { Button, Dropdown } from "antd";
import { ThunderboltOutlined } from "@ant-design/icons";
import type { CsQuickReply } from "@/domain/entities";

interface QuickReplyPickerProps {
  quickReplies: CsQuickReply[];
  onPick: (body: string) => void;
}

/** Canned replies a CS can insert instead of retyping a common answer. */
export function QuickReplyPicker({
  quickReplies,
  onPick,
}: QuickReplyPickerProps) {
  const [open, setOpen] = useState(false);

  const items =
    quickReplies.length > 0
      ? quickReplies.map((reply) => ({
          key: reply.id,
          label: reply.title,
          onClick: () => {
            onPick(reply.body);
            setOpen(false);
          },
        }))
      : [{ key: "empty", label: "Belum ada balasan cepat", disabled: true }];

  return (
    <Dropdown
      menu={{ items }}
      open={open}
      onOpenChange={setOpen}
      trigger={["click"]}
    >
      <Button icon={<ThunderboltOutlined />} title="Balasan cepat" />
    </Dropdown>
  );
}
