import { useState } from "react";
import {
  Button,
  Empty,
  Input,
  List,
  Modal,
  Space,
  Tag,
  Typography,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import type { WaAccount, WaAccountStatus } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

const STATUS_LABEL: Record<WaAccountStatus, string> = {
  connected: "Terhubung",
  disconnected: "Terputus",
  pairing: "Menyambungkan…",
  banned: "Diblokir",
};

const STATUS_COLOR: Record<WaAccountStatus, string> = {
  connected: "success",
  disconnected: "error",
  pairing: "processing",
  banned: "error",
};

/** The number's state as of right now: a live event beats the list this page
 * loaded with, which may be minutes old. */
export function liveStatus(
  account: WaAccount,
  stream: WaStreamStatus,
): WaAccountStatus {
  return stream[account.id]?.waStatus ?? account.status;
}

interface WaNumbersModalProps {
  open: boolean;
  onClose: () => void;
  accounts: WaAccount[];
  stream: WaStreamStatus;
  adding: boolean;
  onAdd: (label: string) => void;
  /** Opens pairing for one number. */
  onPair: (account: WaAccount) => void;
}

/**
 * Every WhatsApp number the team answers from, and the state of each.
 *
 * A number here is a name and nothing more until it is paired — the number
 * itself comes from WhatsApp when the phone approves, so asking an admin to
 * type it in advance would only let the two disagree.
 */
export function WaNumbersModal({
  open,
  onClose,
  accounts,
  stream,
  adding,
  onAdd,
  onPair,
}: WaNumbersModalProps) {
  const [label, setLabel] = useState("");

  const handleAdd = () => {
    const name = label.trim();
    if (!name) return;
    onAdd(name);
    setLabel("");
  };

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      title="Nomor WhatsApp CS"
      width={520}
    >
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Text type="secondary">
          Semua nomor masuk ke satu inbox. Balasan CS otomatis keluar dari nomor
          yang dihubungi pelanggan.
        </Text>

        {accounts.length === 0 ? (
          <Empty description="Belum ada nomor" />
        ) : (
          <List
            size="small"
            dataSource={accounts}
            renderItem={(account) => {
              const status = liveStatus(account, stream);
              return (
                <List.Item
                  actions={[
                    <Button
                      key="pair"
                      size="small"
                      onClick={() => onPair(account)}
                    >
                      {status === "connected" ? "Kelola" : "Sambungkan"}
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    title={<Text strong>{account.label}</Text>}
                    description={
                      <Space size={6}>
                        <Tag color={STATUS_COLOR[status]}>
                          {STATUS_LABEL[status]}
                        </Tag>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          {account.jid?.split("@")[0] ?? "belum terpasang"}
                        </Text>
                      </Space>
                    }
                  />
                </List.Item>
              );
            }}
          />
        )}

        <Space.Compact style={{ width: "100%" }}>
          <Input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            onPressEnter={handleAdd}
            placeholder="Nama nomor baru, misal: CS Teknis"
            maxLength={100}
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleAdd}
            loading={adding}
            disabled={!label.trim()}
          >
            Tambah
          </Button>
        </Space.Compact>
        <Text
          type="secondary"
          style={{ fontSize: 12, color: colors.textMuted }}
        >
          Nomor baru siap dipasang dalam waktu kurang dari satu menit.
        </Text>
      </Space>
    </Modal>
  );
}
