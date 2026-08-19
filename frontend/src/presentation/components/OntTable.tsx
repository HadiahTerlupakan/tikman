import { Table, Tag, Button } from "antd";
import { EyeOutlined } from "@ant-design/icons";
import type { Ont, OntStatus } from "@/domain/entities";

interface OntTableProps {
  dataSource: Ont[];
  isLoading: boolean;
  onViewDetail: (ont: Ont) => void;
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

export function OntTable({ dataSource, isLoading, onViewDetail }: OntTableProps) {
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
          <span>PON Port: <strong>{record.portId}</strong></span>
          <span>ONT ID: <strong>{record.ontId}</strong></span>
        </div>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: OntStatus) => (
        <Tag color={getStatusColor(status)}>{status ? status.toUpperCase() : 'UNKNOWN'}</Tag>
      ),
    },
    {
      title: "Distance (m)",
      key: "distance",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const distance = recordAny.distance;
        return distance > 0 ? distance.toLocaleString() : "-";
      },
    },
    {
      title: "RX Power (dBm)",
      key: "rxPower",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const rxPower = recordAny.rxPower ?? recordAny.metrics?.rxPower;
        return rxPower !== null && rxPower !== undefined ? parseFloat(rxPower).toFixed(2) : "-";
      },
    },
    {
      title: "TX Power (dBm)",
      key: "txPower",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const txPower = recordAny.txPower ?? recordAny.metrics?.txPower;
        return txPower !== null && txPower !== undefined ? parseFloat(txPower).toFixed(2) : "-";
      },
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: Ont) => (
        <Button
          type="link"
          icon={<EyeOutlined />}
          onClick={() => onViewDetail(record)}
        >
          View
        </Button>
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
