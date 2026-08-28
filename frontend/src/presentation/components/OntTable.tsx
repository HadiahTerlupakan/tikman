import { Table, Tag } from "antd";
import { useState } from "react";
import type { Ont, OntStatus } from "@/domain/entities";
import { OntActions } from "./OntActions";
import { ontStatusColor, ontStatusLabel } from "./ontStatus";
import { DEFAULT_ONT_PAGE_SIZE, ontPageSizeOptions } from "./ontPageSize";

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
  onProvision?: (ont: Ont) => void;
  onConfigureService?: (ont: Ont) => void;
  onViewHistory?: (ont: Ont) => void;
}

export function OntTable({
  dataSource,
  isLoading,
  onViewDetail,
  onDelete,
  onProvision,
  onConfigureService,
  onViewHistory,
}: OntTableProps) {
  const [pageSize, setPageSize] = useState(DEFAULT_ONT_PAGE_SIZE);

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
        <Tag color={ontStatusColor(status)}>{ontStatusLabel(status)}</Tag>
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
      width: 132,
      align: "right" as const,
      render: (_: unknown, record: Ont) => (
        <OntActions
          ont={record}
          onViewDetail={onViewDetail}
          onDelete={onDelete}
          onProvision={onProvision}
          onConfigureService={onConfigureService}
          onViewHistory={onViewHistory}
        />
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
        // Held in state and updated on change. Passing pageSize without a
        // handler made it a controlled prop Ant Design could not move, so the
        // size selector reset to five every time it was used.
        pageSize,
        onShowSizeChange: (_, size) => setPageSize(size),
        onChange: (_, size) => setPageSize(size),
        showSizeChanger: true,
        pageSizeOptions: ontPageSizeOptions(dataSource.length),
        showTotal: (total) => `Total ${total} ONTs`,
      }}
    />
  );
}
