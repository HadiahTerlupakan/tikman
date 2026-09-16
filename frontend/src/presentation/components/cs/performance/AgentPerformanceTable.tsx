import { Card, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { AgentPerformance } from "@/domain/entities";
import { formatMinutes, formatPercent } from "./performanceFormat";

interface AgentPerformanceTableProps {
  agents: AgentPerformance[];
  targetMinutes: number;
  loading: boolean;
  onSelect: (agent: AgentPerformance) => void;
}

/** One row per CS: how fast the replies they sent were, and how many threads
 * they closed without sending one. Anyone but an admin is served their own row
 * alone, by the API. */
export function AgentPerformanceTable({
  agents,
  targetMinutes,
  loading,
  onSelect,
}: AgentPerformanceTableProps) {
  const columns: ColumnsType<AgentPerformance> = [
    {
      title: "CS",
      dataIndex: "username",
      render: (username: string) => username || "Pengguna terhapus",
    },
    { title: "Balasan", render: (_, agent) => agent.replies.count },
    {
      title: "Median",
      render: (_, agent) => formatMinutes(agent.replies.medianMinutes),
    },
    {
      title: `≤ ${targetMinutes} menit`,
      render: (_, agent) => formatPercent(agent.replies.withinTargetPct),
    },
    {
      title: "90% tercepat",
      render: (_, agent) => formatMinutes(agent.replies.p90Minutes),
    },
    { title: "Ditutup tanpa balasan", dataIndex: "closedWithoutReply" },
  ];

  return (
    <Card title="Per CS">
      <Table
        rowKey="userId"
        size="small"
        scroll={{ x: 720 }}
        loading={loading}
        dataSource={agents}
        columns={columns}
        pagination={false}
        onRow={(agent) => ({
          onClick: () => onSelect(agent),
          style: { cursor: "pointer" },
        })}
      />
    </Card>
  );
}
