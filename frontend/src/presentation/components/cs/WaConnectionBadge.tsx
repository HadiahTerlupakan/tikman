import { Tag } from "antd";
import { CheckCircleOutlined, WarningOutlined } from "@ant-design/icons";
import type { WaAccountStatus } from "@/domain/entities";

interface WaConnectionBadgeProps {
  status?: WaAccountStatus;
  /** Present only for an admin — clicking opens the pairing panel. Omitted
   * for every other role: they need to see the state, not a button the
   * server will refuse. */
  onOpenPairing?: () => void;
}

const STATUS_LABEL: Record<WaAccountStatus, string> = {
  connected: "WhatsApp Terhubung",
  disconnected: "WhatsApp Terputus",
  pairing: "Menyambungkan WhatsApp...",
  banned: "Nomor WhatsApp Diblokir",
};

/**
 * Sits at the top of the inbox because a disconnected number means every
 * reply a CS types is stored and never delivered — that has to be obvious
 * before anyone starts typing, not discovered after a customer complains.
 */
export function WaConnectionBadge({
  status,
  onOpenPairing,
}: WaConnectionBadgeProps) {
  const connected = status === "connected";
  const label = status ? STATUS_LABEL[status] : "Memeriksa koneksi WhatsApp...";
  const color = connected ? "success" : status ? "error" : "default";

  return (
    <Tag
      icon={connected ? <CheckCircleOutlined /> : <WarningOutlined />}
      color={color}
      onClick={onOpenPairing}
      style={{
        cursor: onOpenPairing ? "pointer" : "default",
        fontSize: 13,
        padding: "4px 10px",
      }}
    >
      {label}
    </Tag>
  );
}
