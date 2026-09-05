import { Table, Tag } from "antd";
import type { Odp, Ont, OntStatus } from "@/domain/entities";
import { OntActions } from "./OntActions";
import { ontStatusColor, ontStatusLabel } from "./ontStatus";
import { ontPageSizeOptions } from "./ontPageSize";

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
  /** The page being shown, and how many rows match in total on the server. */
  page: number;
  total: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  onViewDetail: (ont: Ont) => void;
  onDelete: (id: string) => void;
  onProvision?: (ont: Ont) => void;
  onConfigureService?: (ont: Ont) => void;
  onViewHistory?: (ont: Ont) => void;
  onSetOdp?: (ont: Ont) => void;
  /** The plant, so a row can name the box it lands in rather than its id. */
  odps?: Odp[];
}

export function OntTable({
  dataSource,
  isLoading,
  page,
  total,
  pageSize,
  onPageChange,
  onPageSizeChange,
  onViewDetail,
  onDelete,
  onProvision,
  onConfigureService,
  onViewHistory,
  onSetOdp,
  odps,
}: OntTableProps) {
  const odpCodes = new Map((odps ?? []).map((odp) => [odp.id, odp.code]));
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
      title: "Kartu/PON Port/ONT ID",
      key: "position",
      render: (_: unknown, record: Ont) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          {/* Port numbers repeat across cards, so the port alone does not name
              an ONU. Omitted rather than dashed where the OLT never reported a
              card: an empty label reads as card zero. */}
          {record.slot !== undefined && (
            <span>
              Kartu: <strong>{record.slot}</strong>
            </span>
          )}
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
      title: "ODP",
      key: "odp",
      render: (_: unknown, record: Ont) =>
        record.odpId
          ? `${odpCodes.get(record.odpId) ?? "ODP"} · port ${record.odpPort}`
          : "-",
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
          onSetOdp={onSetOdp}
        />
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      scroll={{ x: 1260 }}
      dataSource={dataSource}
      rowKey="id"
      loading={isLoading}
      pagination={{
        position: ["bottomCenter"],
        // Driven entirely by the caller, because the rows in hand are one page
        // of a result the database holds. Letting Ant Design page over
        // dataSource would page over the current page.
        current: page,
        pageSize,
        total,
        onChange: (nextPage, size) => {
          if (size !== pageSize) {
            onPageSizeChange(size);
            return;
          }
          onPageChange(nextPage);
        },
        onShowSizeChange: (_, size) => onPageSizeChange(size),
        showSizeChanger: true,
        pageSizeOptions: ontPageSizeOptions(total),
        showTotal: (shown) => `Total ${shown} ONTs`,
      }}
    />
  );
}
