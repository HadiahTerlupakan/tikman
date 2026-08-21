import { Table, Button, Space, Tag, Popconfirm, Progress } from "antd";
import { EditOutlined, DeleteOutlined, SyncOutlined } from "@ant-design/icons";
import { type Olt, OltStatus } from "@/domain/entities";
import type { ColumnsType } from "antd/es/table";
import { useEffect, useState } from "react";
import { apiClient } from "@/infrastructure/http/apiClient";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";

interface OltTableProps {
  olts: Olt[];
  loading: boolean;
  onEdit: (olt: Olt) => void;
  onDelete: (id: string) => void;
}

interface OLTStats {
  totalOnts: number;
  ontsWithMetrics: number;
  percentage: number;
  lastPollTime?: string;
  oltId?: string;
}

export function OltTable({ olts, loading, onEdit, onDelete }: OltTableProps) {
  const [oltStats, setOltStats] = useState<Record<string, OLTStats>>({});

  useEffect(() => {
    if (olts.length === 0) return;

    const fetchOltsStats = async () => {
      const stats: Record<string, OLTStats> = {};
      for (const olt of olts) {
        try {
          const response = await apiClient.get(`${API_ENDPOINTS.OLTS}/${olt.id}/stats`);
          stats[olt.id] = response.data;
        } catch (error) {
          console.error(`Failed to fetch stats for OLT ${olt.id}:`, error);
          stats[olt.id] = { totalOnts: 0, ontsWithMetrics: 0, percentage: 0 };
        }
      }
      setOltStats(stats);
    };

    fetchOltsStats();
    const intervalId = setInterval(fetchOltsStats, 15000);
    return () => clearInterval(intervalId);
  }, [olts]);

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
      render: (_, record) => {
        const stats = oltStats[record.id] || { totalOnts: 0, ontsWithMetrics: 0, percentage: 0 };

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
              {stats.percentage < 100 && (
                <SyncOutlined spin style={{ color: "#7dd3fc", fontSize: 12 }} />
              )}
              {stats.lastPollTime && (
                <span style={{ fontSize: 11, color: "#4ade80" }}>
                  Updated: {new Date(stats.lastPollTime).toLocaleTimeString()}
                </span>
              )}
            </div>
          </div>
        );
      },
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
