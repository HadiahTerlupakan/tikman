import { Table, Button, Space, Tag, Popconfirm } from "antd";
import { EditOutlined, DeleteOutlined } from "@ant-design/icons";
import { type Olt, OltStatus } from "@/domain/entities";
import type { ColumnsType } from "antd/es/table";

interface OltTableProps {
  olts: Olt[];
  loading: boolean;
  onEdit: (olt: Olt) => void;
  onDelete: (id: string) => void;
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
        <Tag color={getStatusColor(status)}>{status.toUpperCase()}</Tag>
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
      scroll={{ x: 1000 }}
    />
  );
}
