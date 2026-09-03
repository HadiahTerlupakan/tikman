import { CheckOutlined, ClockCircleOutlined } from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

/**
 * How far a reply got, in the shorthand a CS already reads on their phone: one
 * tick left the app, two arrived, two in colour were read. A queued reply shows
 * a clock, because "sent" would be a lie while it is still waiting.
 */
export function DeliveryMark({ status }: { status: CsMessage["status"] }) {
  const marks: Partial<
    Record<CsMessage["status"], { icon: JSX.Element; label: string }>
  > = {
    queued: {
      icon: <ClockCircleOutlined style={{ fontSize: 11 }} />,
      label: "Menunggu dikirim",
    },
    sent: {
      icon: <CheckOutlined style={{ fontSize: 11 }} />,
      label: "Terkirim",
    },
    delivered: {
      icon: (
        <span style={{ letterSpacing: -4 }}>
          <CheckOutlined style={{ fontSize: 11 }} />
          <CheckOutlined style={{ fontSize: 11 }} />
        </span>
      ),
      label: "Sampai di HP pelanggan",
    },
    read: {
      icon: (
        <span style={{ letterSpacing: -4, color: colors.success }}>
          <CheckOutlined style={{ fontSize: 11 }} />
          <CheckOutlined style={{ fontSize: 11 }} />
        </span>
      ),
      label: "Dibaca",
    },
  };

  const mark = marks[status];
  if (!mark) return null;
  return (
    <span aria-label={mark.label} title={mark.label}>
      {mark.icon}
    </span>
  );
}
