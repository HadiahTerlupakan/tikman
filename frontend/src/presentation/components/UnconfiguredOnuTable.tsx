import { Table, Typography, Button, Tooltip } from "antd";
import { CopyOutlined, PlusOutlined } from "@ant-design/icons";
import type { DetectedOnu } from "@/domain/entities";

interface UnconfiguredOnuTableProps {
  dataSource: DetectedOnu[];
  isLoading: boolean;
  /** Off when the list is filtered to one OLT, where the column repeats itself. */
  showOlt?: boolean;
  onCopySerial: (serialNumber: string) => void;
  onRegister?: (record: DetectedOnu) => void;
}

export function UnconfiguredOnuTable({
  dataSource,
  isLoading,
  showOlt,
  onCopySerial,
  onRegister,
}: UnconfiguredOnuTableProps) {
  const columns = [
    ...(showOlt
      ? [
          {
            title: "OLT",
            dataIndex: "oltName",
            key: "oltName",
          },
        ]
      : []),
    {
      title: "PON Port",
      key: "location",
      render: (_: unknown, record: DetectedOnu) =>
        `${record.slot}/${record.port}`,
    },
    {
      title: "Serial Number",
      dataIndex: "serialNumber",
      key: "serialNumber",
      render: (serialNumber: string) => (
        <Typography.Text strong>{serialNumber}</Typography.Text>
      ),
    },
    {
      title: "Device Type",
      dataIndex: "deviceType",
      key: "deviceType",
      render: (deviceType?: string) => deviceType || "-",
    },
    {
      title: "Software Version",
      dataIndex: "softwareVersion",
      key: "softwareVersion",
      render: (softwareVersion?: string) => softwareVersion || "-",
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: DetectedOnu) => (
        <>
          <Tooltip title="Copy serial number">
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => onCopySerial(record.serialNumber)}
              aria-label={`Copy serial number ${record.serialNumber}`}
            />
          </Tooltip>
          {onRegister && (
            <Button
              size="small"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => onRegister(record)}
            >
              Register
            </Button>
          )}
        </>
      ),
    },
  ];

  return (
    <Table
      rowKey={(record) =>
        `${record.oltId}-${record.slot}-${record.port}-${record.serialNumber}`
      }
      columns={columns}
      dataSource={dataSource}
      loading={isLoading}
      pagination={false}
      scroll={{ x: 720 }}
      locale={{ emptyText: "No unconfigured ONUs detected" }}
    />
  );
}
