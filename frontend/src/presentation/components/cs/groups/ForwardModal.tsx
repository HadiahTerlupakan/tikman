import { useEffect, useState } from "react";
import { Modal, Select, Typography } from "antd";
import type { WaGroup } from "@/domain/entities";

const { Text } = Typography;

/** Picks the groups a message is copied to. Only groups our number can still
 * post in are offered; the copy goes out marked as forwarded. */
export function ForwardModal({
  open,
  groups,
  forwarding,
  onForward,
  onClose,
}: {
  open: boolean;
  groups: WaGroup[];
  forwarding: boolean;
  onForward: (groupIds: string[]) => void;
  onClose: () => void;
}) {
  const [picked, setPicked] = useState<string[]>([]);
  useEffect(() => {
    if (open) setPicked([]);
  }, [open]);

  const options = groups
    .filter((g) => !g.left && (!g.announce || g.isAdmin))
    .map((g) => ({ value: g.id, label: g.name || g.jid }));

  return (
    <Modal
      open={open}
      title="Teruskan pesan"
      okText="Teruskan"
      cancelText="Batal"
      okButtonProps={{ disabled: picked.length === 0, loading: forwarding }}
      onOk={() => onForward(picked)}
      onCancel={onClose}
      destroyOnHidden
    >
      <Select
        mode="multiple"
        autoFocus
        style={{ width: "100%" }}
        placeholder="Pilih grup tujuan"
        value={picked}
        onChange={setPicked}
        options={options}
        optionFilterProp="label"
      />
      <Text
        type="secondary"
        style={{ display: "block", fontSize: 12, marginTop: 6 }}
      >
        Pesan dikirim dari nomor yang sama dengan label "Diteruskan".
      </Text>
    </Modal>
  );
}
