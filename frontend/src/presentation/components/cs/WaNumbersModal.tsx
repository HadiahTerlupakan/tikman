import { useState } from "react";
import {
  Button,
  Dropdown,
  Empty,
  Input,
  List,
  Modal,
  Space,
  Tag,
  Typography,
} from "antd";
import {
  ClearOutlined,
  DeleteOutlined,
  MoreOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import type { WaAccount, WaAccountStatus } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";
import { colors } from "@/shared/theme/colors";
import { ConfirmByTypingModal } from "./ConfirmByTypingModal";

const { Text } = Typography;

/** What an admin types to empty every thread on every number. */
const CLEAR_INBOX_PHRASE = "HAPUS SEMUA";

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
  busy: boolean;
  onAdd: (label: string) => void;
  /** Opens pairing for one number. */
  onPair: (account: WaAccount) => void;
  /** Empties one number's threads without removing the number. */
  onClearMessages: (account: WaAccount) => void;
  /** Removes a number along with every thread, message and file on it. */
  onDelete: (account: WaAccount) => void;
  onClearInbox: () => void;
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
  busy,
  onAdd,
  onPair,
  onClearMessages,
  onDelete,
  onClearInbox,
}: WaNumbersModalProps) {
  const [label, setLabel] = useState("");
  const [doomed, setDoomed] = useState<WaAccount>();
  const [clearingAccount, setClearingAccount] = useState<WaAccount>();
  const [clearingInbox, setClearingInbox] = useState(false);

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
                    <Dropdown
                      key="more"
                      trigger={["click"]}
                      menu={{
                        items: [
                          {
                            key: "clear",
                            icon: <ClearOutlined />,
                            label: "Bersihkan pesan",
                            onClick: () => setClearingAccount(account),
                          },
                          {
                            key: "delete",
                            icon: <DeleteOutlined />,
                            danger: true,
                            label: "Hapus nomor",
                            onClick: () => setDoomed(account),
                          },
                        ],
                      }}
                    >
                      <Button
                        size="small"
                        type="text"
                        icon={<MoreOutlined />}
                        aria-label={`Kelola ${account.label}`}
                      />
                    </Dropdown>,
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

        <div
          style={{ borderTop: `1px solid ${colors.border}`, paddingTop: 12 }}
        >
          <Button
            danger
            size="small"
            icon={<ClearOutlined />}
            onClick={() => setClearingInbox(true)}
          >
            Bersihkan seluruh inbox
          </Button>
        </div>
      </Space>

      {/* Its own modal rather than a Popconfirm on the menu item: a Dropdown
          closes on click and takes the popconfirm down with it, so the
          confirmation flashed and vanished before it could be answered. */}
      <Modal
        open={!!clearingAccount}
        title={`Bersihkan pesan ${clearingAccount?.label ?? ""}?`}
        okText="Bersihkan"
        okButtonProps={{ danger: true, loading: busy }}
        cancelText="Batal"
        onOk={() => {
          if (clearingAccount) onClearMessages(clearingAccount);
          setClearingAccount(undefined);
        }}
        onCancel={() => setClearingAccount(undefined)}
        width={460}
      >
        <Text type="secondary">
          Semua riwayat percakapan di nomor ini dihapus di TikMan. Nomornya
          tetap tersambung dan percakapannya tetap ada.
        </Text>
      </Modal>

      <ConfirmByTypingModal
        open={!!doomed}
        title={`Hapus nomor ${doomed?.label ?? ""}?`}
        warning="Semua percakapan, pesan dan lampiran di nomor ini ikut terhapus, dan tidak bisa dikembalikan. Pairing di HP juga dilepas."
        phrase={doomed?.label ?? ""}
        confirmText="Hapus nomor"
        loading={busy}
        onConfirm={() => {
          if (doomed) onDelete(doomed);
          setDoomed(undefined);
        }}
        onClose={() => setDoomed(undefined)}
      />

      <ConfirmByTypingModal
        open={clearingInbox}
        title="Bersihkan seluruh inbox?"
        warning="Semua pesan di semua nomor dihapus, dan tidak bisa dikembalikan. Nomor dan daftar percakapannya tetap ada."
        phrase={CLEAR_INBOX_PHRASE}
        confirmText="Bersihkan semua"
        loading={busy}
        onConfirm={() => {
          onClearInbox();
          setClearingInbox(false);
        }}
        onClose={() => setClearingInbox(false)}
      />
    </Modal>
  );
}
