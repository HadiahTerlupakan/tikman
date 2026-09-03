import { useState } from "react";
import { Button, Descriptions, Popconfirm, Space, Tag, Typography } from "antd";
import { DisconnectOutlined, EyeOutlined } from "@ant-design/icons";
import type { Ont } from "@/domain/entities";
import { OntDetailModal } from "@/presentation/components/OntDetailModal";
import { ontAddressLabel } from "@/presentation/components/ontAddress";
import {
  ontStatusColor,
  ontStatusLabel,
} from "@/presentation/components/ontStatus";
import {
  rxSignalQuality,
  txSignalQuality,
} from "@/presentation/components/ont-detail/signalQuality";

const { Text } = Typography;

/** A reading with the colour a technician already reads it by. Absent is not
 * zero: an ONT that has reported nothing shows a dash, because "0.00 dBm"
 * would be a measurement nobody took. */
function Power({
  value,
  quality,
}: {
  value?: number | null;
  quality: (power: number) => { label: string; color: string };
}) {
  if (value === null || value === undefined)
    return <Text type="secondary">—</Text>;
  return <Tag color={quality(value).color}>{Number(value).toFixed(2)} dBm</Tag>;
}

interface OntLinkPanelProps {
  ont?: Ont;
  loading: boolean;
  unlinking: boolean;
  onUnlink: () => void;
}

/**
 * The ONT a thread is linked to.
 *
 * It carries the readings a CS is actually asked about — a customer reports a
 * slow line, and the answer is usually in the optical power — rather than
 * status alone, which says a link is up without saying how well. The address
 * is shown the way the OLT's own CLI writes it, so a CS reading it to a
 * technician is reading the same thing the technician will type.
 */
export function OntLinkPanel({
  ont,
  loading,
  unlinking,
  onUnlink,
}: OntLinkPanelProps) {
  const [detailOpen, setDetailOpen] = useState(false);

  if (loading || !ont) {
    return <Text type="secondary">Memuat ONT…</Text>;
  }

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="small">
      <Space wrap>
        <Tag color={ontStatusColor(ont.status)}>
          {ontStatusLabel(ont.status)}
        </Tag>
        <Text strong>{ont.name || ont.serialNumber}</Text>
      </Space>

      <Descriptions
        size="small"
        column={1}
        colon={false}
        labelStyle={{ width: 68 }}
        items={[
          {
            key: "rx",
            label: "Redaman",
            children: (
              <Space size={4}>
                <Text type="secondary">RX</Text>
                <Power value={ont.rxPower} quality={rxSignalQuality} />
                <Text type="secondary">TX</Text>
                <Power value={ont.txPower} quality={txSignalQuality} />
              </Space>
            ),
          },
          {
            key: "olt",
            label: "OLT",
            children: <Text>{ont.oltName}</Text>,
          },
          {
            key: "posisi",
            // rack/card/pon:onu, the address the OLT's CLI uses.
            label: "Posisi",
            children: (
              <Text code>
                {ontAddressLabel(ont.slot, ont.portId, ont.ontId)}
              </Text>
            ),
          },
        ]}
      />

      <Space>
        <Button
          size="small"
          icon={<EyeOutlined />}
          onClick={() => setDetailOpen(true)}
        >
          Detail
        </Button>
        {/* Confirmed, because unlinking also takes the customer's number back
            off the ONT — a misclick would undo data entry nobody asked to
            undo. */}
        <Popconfirm
          title="Lepas tautan ONT?"
          description="Nomor pelanggan ini juga dilepas dari ONT tersebut."
          okText="Lepas"
          cancelText="Batal"
          onConfirm={onUnlink}
        >
          <Button
            size="small"
            danger
            icon={<DisconnectOutlined />}
            loading={unlinking}
          >
            Lepas
          </Button>
        </Popconfirm>
      </Space>

      <OntDetailModal
        ont={ont}
        visible={detailOpen}
        onClose={() => setDetailOpen(false)}
      />
    </Space>
  );
}
