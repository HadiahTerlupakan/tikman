import { Table, Tag, Empty, Skeleton } from "antd";
import type { ColumnsType } from "antd/es/table";
import { OltStatus } from "@/domain/entities";
import { colors, statusSurfaces } from "@/shared/theme";
import type { OltBreakdown } from "@/presentation/pages/dashboardStats";
import { availabilityTone } from "@/presentation/pages/dashboardStats";

interface OltBreakdownTableProps {
  rows: OltBreakdown[];
  isLoading?: boolean;
}

const OLT_STATUS_TONE: Record<OltStatus, "success" | "neutral" | "danger"> = {
  [OltStatus.ONLINE]: "success",
  [OltStatus.OFFLINE]: "neutral",
  [OltStatus.ERROR]: "danger",
};

/**
 * Per-OLT counts derived from the ONT list already on the page. An aggregate
 * "12 offline" tells an operator that something is wrong; this tells them where
 * to go, which is the only part they can act on.
 */
export function OltBreakdownTable({ rows, isLoading }: OltBreakdownTableProps) {
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 4 }} title={false} />;
  }

  if (rows.length === 0) {
    return (
      <Empty
        description={
          <span style={{ color: colors.textSecondary }}>
            No OLTs registered yet
          </span>
        }
      />
    );
  }

  return (
    <Table<OltBreakdown>
      dataSource={rows}
      columns={columns}
      rowKey="oltId"
      size="small"
      pagination={false}
      scroll={{ x: 520 }}
    />
  );
}

const columns: ColumnsType<OltBreakdown> = [
  {
    title: "OLT",
    dataIndex: "oltName",
    render: (name: string, row) => (
      <div style={{ display: "flex", flexDirection: "column" }}>
        <span style={{ color: colors.textPrimary }}>{name}</span>
        <span style={{ color: colors.textMuted, fontSize: 11 }}>
          {row.ontTotal === 0 ? "no ONTs" : `${row.ontTotal} ONTs`}
        </span>
      </div>
    ),
  },
  {
    title: "Status",
    dataIndex: "oltStatus",
    width: 110,
    render: (status: OltStatus) => (
      <Tag color={statusSurfaces[OLT_STATUS_TONE[status]].accent}>{status}</Tag>
    ),
  },
  {
    title: "Online",
    dataIndex: "online",
    width: 90,
    align: "right",
    render: (online: number) => <Figure value={online} tone="success" />,
  },
  {
    title: "Impaired",
    dataIndex: "impaired",
    width: 100,
    align: "right",
    // Zero impaired carries no alarm, so it is not painted like one.
    render: (impaired: number) => (
      <Figure value={impaired} tone={impaired > 0 ? "danger" : "quiet"} />
    ),
  },
  {
    title: "Availability",
    dataIndex: "availability",
    width: 120,
    align: "right",
    render: (availability: number | null) =>
      availability === null ? (
        <span style={{ color: colors.textMuted }}>—</span>
      ) : (
        <Figure
          value={`${availability}%`}
          tone={availabilityTone(availability)}
        />
      ),
  },
];

function Figure({
  value,
  tone,
}: {
  value: number | string;
  tone: keyof typeof statusSurfaces;
}) {
  return (
    <span
      style={{
        color: statusSurfaces[tone].accent,
        fontVariantNumeric: "tabular-nums",
      }}
    >
      {value}
    </span>
  );
}
