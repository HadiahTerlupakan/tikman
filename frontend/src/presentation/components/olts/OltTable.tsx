import { Table, Button, Space, Tag, Popconfirm, Progress } from "antd";
import { EditOutlined, DeleteOutlined, SyncOutlined } from "@ant-design/icons";
import { type Olt, OltStatus, type OltStats } from "@/domain/entities";
import type { ColumnsType } from "antd/es/table";
import { useOltStats } from "@/application/hooks";

interface OltTableProps {
  olts: Olt[];
  loading: boolean;
  onEdit: (olt: Olt) => void;
  onDelete: (id: string) => void;
}

const emptyStats: OltStats = { totalOnts: 0, ontsWithMetrics: 0, percentage: 0 };

function OltMetricsCell({ oltId }: { oltId: string }) {
  const { data: stats = emptyStats } = useOltStats(oltId);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <Progress
        percent={stats.percentage}
        size="small"
        strokeColor={stats.percentage === 100 ? "#3ecf8e" : "#7dd3fc"}
        format={() => `${Math.round(stats.percentage)}%`}
        showInfo={false}
      />
      <span style={{ fontSize: 11, color: "#94a3b8" }}>
        {stats.ontsWithMetrics}/{stats.totalOnts} ONTs polled
      </span>
      <div style={{ display: "flex", alignItems: "center", gap: 4, marginTop: 2 }}>
        {stats.percentage < 100 && <SyncOutlined spin style={{ color: "#7dd3fc", fontSize: 12 }} />}
        {stats.lastPollTime && (
          <span style={{ fontSize: 11, color: "#4ade80" }}>
            Updated: {new Date(stats.lastPollTime).toLocaleTimeString()}
          </span>
        )}
      </div>
    </div>
  );
}

export function OltTable({ olts, loading, onEdit, onDelete }: OltTableProps) {
  const getStatusColor = (status: OltStatus) => {
    switch (status) {
      case OltStatus.ONLINE:
        return "green";
      case OltStatus.OFFLINE:
        return "red";
      case OltStatus.ERROR:
        return "orange";
      default:
        return "default";
    }
  };

  const columns: ColumnsType<Olt> = [
    {
      title: "OLT Name",
      dataIndex: "name",
      key: "name",
      fixed: "left",
      width: 180,
    },
    {
      title: "Site",
      dataIndex: "siteName",
      key: "siteName",
      width: 150,
      responsive: ["md"],
    },
    {
      title: "IP Address",
      dataIndex: "ipAddress",
      key: "ipAddress",
      width: 150,
    },
    {
      title: "Protocol",
      dataIndex: "preferredProtocol",
      key: "preferredProtocol",
      width: 100,
      responsive: ["lg"],
      render: (protocol: string) => protocol.toUpperCase(),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      width: 120,
      render: (status: OltStatus) => (
        <Tag color={getStatusColor(status)}>{status ? status.toUpperCase() : 'UNKNOWN'}</Tag>
      ),
    },
    {
      title: "Last Seen",
      dataIndex: "lastSeen",
      key: "lastSeen",
      width: 180,
      responsive: ["xl"],
      render: (date: string | null) =>
        date ? new Date(date).toLocaleString("id-ID") : "-",
    },
    {
      title: "ONT Metrics",
      key: "metrics",
      width: 250,
      render: (_, record) => <OltMetricsCell oltId={record.id} />,
    },
    {
      title: "Actions",
      key: "actions",
      fixed: "right",
      width: 180,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => onEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Hapus OLT ini?"
            onConfirm={() => onDelete(record.id)}
            okText="Ya"
            cancelText="Tidak"
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={olts}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
      scroll={{ x: 1200 }}
    />
  );
}
