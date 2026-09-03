import { Tag } from "antd";
import { CheckCircleOutlined, WarningOutlined } from "@ant-design/icons";
import type { WaAccount } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";
import { liveStatus } from "./WaNumbersModal";

interface WaConnectionBadgeProps {
  accounts?: WaAccount[];
  stream: WaStreamStatus;
  /** Present only for an admin — clicking opens the numbers panel. Omitted
   * for every other role: they need to see the state, not a button the
   * server will refuse. */
  onOpenNumbers?: () => void;
}

/**
 * Sits at the top of the inbox because a disconnected number means every
 * reply a CS types on its threads is stored and never delivered — that has to
 * be obvious before anyone starts typing, not discovered after a customer
 * complains.
 *
 * With several numbers it counts rather than names: one down out of six is
 * still a problem, and the panel behind it says which. It stays clickable for
 * an admin with no numbers at all: that panel is the only way to add one, so
 * an inert badge there is a dead end after the last number is deleted.
 */
export function WaConnectionBadge({
  accounts,
  stream,
  onOpenNumbers,
}: WaConnectionBadgeProps) {
  if (!accounts) {
    return (
      <Tag style={{ fontSize: 13, padding: "4px 10px" }}>
        Memeriksa koneksi WhatsApp…
      </Tag>
    );
  }

  const connected = accounts.filter(
    (account) => liveStatus(account, stream) === "connected",
  ).length;
  // No numbers is not "nothing wrong": it is where deleting the last one
  // lands, and nothing can be answered until another is paired. Counting it
  // as up would also read as "Terhubung (0)".
  const allUp = accounts.length > 0 && connected === accounts.length;
  const label =
    accounts.length === 0
      ? "Belum ada nomor WhatsApp"
      : allUp
        ? `WhatsApp Terhubung (${connected})`
        : `${accounts.length - connected} dari ${accounts.length} nomor terputus`;

  return (
    <Tag
      icon={allUp ? <CheckCircleOutlined /> : <WarningOutlined />}
      color={allUp ? "success" : "error"}
      onClick={onOpenNumbers}
      style={{
        cursor: onOpenNumbers ? "pointer" : "default",
        fontSize: 13,
        padding: "4px 10px",
      }}
    >
      {label}
    </Tag>
  );
}
