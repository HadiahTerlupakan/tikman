import { Table, Tag, Button, Popconfirm, Space } from "antd";
import { DeleteOutlined, EyeOutlined } from "@ant-design/icons";
import type { Ont, OntStatus } from "@/domain/entities";

interface OntTableRow extends Ont {
  metrics?: {
    rxPower?: number | null;
    txPower?: number | null;
    distance?: number | null;
  };
}

interface OntTableProps {
  dataSource: Ont[];
  isLoading: boolean;
  onViewDetail: (ont: Ont) => void;
  onDelete: (id: string) => void;
}

const getStatusColor = (status: OntStatus) => {
  switch (status) {
    case "online":
      return "success";
    case "offline":
      return "default";
    case "los":
      return "warning";
    case "dying_gasp":
      return "error";
    case "unknown":
      return "default";
    default:
      return "default";
  }
};

export function OntTable({
  dataSource,
  isLoading,
  onViewDetail,
  onDelete,
}: OntTableProps) {
  const columns = [
    {
      title: "Serial Number",
      dataIndex: "serialNumber",
      key: "serialNumber",
    },
    {
      title: "Name",
      dataIndex: "name",
      key: "name",
      render: (name: string) => name || "-",
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      render: (description: string) => description || "-",
    },
    {
      title: "OLT",
      dataIndex: "oltName",
      key: "oltName",
    },
    {
      title: "PON Port/ONT ID",
      key: "position",
      render: (_: unknown, record: Ont) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span>
            PON Port: <strong>{record.portId}</strong>
          </span>
          <span>
            ONT ID: <strong>{record.ontId}</strong>
          </span>
        </div>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: OntStatus) => (
        <Tag color={getStatusColor(status)}>
          {status ? status.toUpperCase() : "UNKNOWN"}
        </Tag>
      ),
    },
    {
      title: "Distance (m)",
      key: "distance",
      render: (_: unknown, record: Ont) => {
        const distance =
          (record as OntTableRow).distance ??
          (record as OntTableRow).metrics?.distance;
        return distance !== null && distance !== undefined && distance > 0
          ? distance.toLocaleString()
          : "-";
      },
    },
    {
      title: "RX Power (dBm)",
      key: "rxPower",
      render: (_: unknown, record: Ont) => {
        const rxPower =
          (record as OntTableRow).rxPower ??
          (record as OntTableRow).metrics?.rxPower;
        return rxPower !== null && rxPower !== undefined
          ? Number(rxPower).toFixed(2)
          : "-";
      },
    },
    {
      title: "TX Power (dBm)",
      key: "txPower",
      render: (_: unknown, record: Ont) => {
        const txPower =
          (record as OntTableRow).txPower ??
          (record as OntTableRow).metrics?.txPower;
        return txPower !== null && txPower !== undefined
          ? Number(txPower).toFixed(2)
          : "-";
      },
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: Ont) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => onViewDetail(record)}
          >
            View
          </Button>
          <Popconfirm
            title="Hapus ONT ini?"
            description="Data metrics dan event ONT ini juga akan dihapus"
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
      dataSource={dataSource}
      rowKey="id"
      loading={isLoading}
      pagination={{
        position: ["bottomCenter"],
        total: dataSource.length,
        pageSize: 5,
        showSizeChanger: true,
        pageSizeOptions: ["5", "10", "20", "all"],
        showTotal: (total) => `Total ${total} ONTs`,
      }}
    />
  );
}
