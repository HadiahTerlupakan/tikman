import { useState } from "react";
import { Drawer, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useNavigate } from "react-router-dom";
import { useCsPerformanceWaits } from "@/application/hooks/useCsPerformance";
import type { PerformancePeriod, PerformanceWait } from "@/domain/entities";
import { END_REASON_LABELS, formatMinutes } from "./performanceFormat";

const PAGE_SIZE = 20;

interface WaitListDrawerProps {
  title: string;
  period: PerformancePeriod;
  userId?: string;
  onClose: () => void;
}

/** Times are read in WIB, the way the report counts its days. */
function wibTime(iso: string): string {
  return new Date(iso).toLocaleString("id-ID", {
    timeZone: "Asia/Jakarta",
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** The waits behind the figures, newest first, so a CS can check the number
 * they were given and open the thread it came from. */
export function WaitListDrawer({
  title,
  period,
  userId,
  onClose,
}: WaitListDrawerProps) {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const { data, isLoading } = useCsPerformanceWaits({
    ...period,
    userId,
    limit: PAGE_SIZE,
    offset: (page - 1) * PAGE_SIZE,
  });

  const columns: ColumnsType<PerformanceWait> = [
    {
      title: "Pelanggan",
      render: (_, wait) =>
        wait.conversationDeleted ? (
          <Tag>Thread dihapus</Tag>
        ) : (
          wait.customerName || wait.customerPhone
        ),
    },
    { title: "Mulai", render: (_, wait) => wibTime(wait.startedAt) },
    { title: "Selesai", render: (_, wait) => wibTime(wait.endedAt) },
    {
      title: "Cara selesai",
      render: (_, wait) => END_REASON_LABELS[wait.endReason],
    },
    { title: "Oleh", render: (_, wait) => wait.endedByUsername ?? "—" },
    {
      title: "Menit tim",
      render: (_, wait) => formatMinutes(wait.teamMinutes),
    },
    {
      title: "Menit dihitung",
      render: (_, wait) => formatMinutes(wait.countedMinutes),
    },
    {
      title: "Catatan",
      render: (_, wait) =>
        wait.systemDelayed ? <Tag color="warning">Tertunda sistem</Tag> : null,
    },
  ];

  return (
    // antd's Drawer panel carries role="dialog" but wires no aria-labelledby
    // to its visible title, so a screen reader (and an accessible-name-based
    // query) would otherwise hear only "dialog" — aria-label gives it the
    // same name a sighted user reads.
    <Drawer open title={title} aria-label={title} width={960} onClose={onClose}>
      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 900 }}
        loading={isLoading}
        dataSource={data?.items}
        columns={columns}
        pagination={{
          current: page,
          pageSize: PAGE_SIZE,
          total: data?.total ?? 0,
          onChange: setPage,
          showSizeChanger: false,
        }}
        onRow={(wait) => ({
          onClick: () => {
            if (!wait.conversationDeleted) {
              navigate(`/cs?conversation=${wait.conversationId}`);
            }
          },
          style: { cursor: wait.conversationDeleted ? "default" : "pointer" },
        })}
      />
    </Drawer>
  );
}
