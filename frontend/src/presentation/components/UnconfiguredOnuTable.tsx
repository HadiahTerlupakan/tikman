import { Table, Typography, Button, Tooltip } from "antd";
import { CopyOutlined, PlusOutlined } from "@ant-design/icons";
import type { UnconfiguredOnu } from "@/domain/entities";

interface UnconfiguredOnuTableProps {
  dataSource: UnconfiguredOnu[];
  isLoading: boolean;
  onCopySerial: (serialNumber: string) => void;
  onRegister?: (record: UnconfiguredOnu) => void;
}

export function UnconfiguredOnuTable({
  dataSource,
  isLoading,
  onCopySerial,
  onRegister,
}: UnconfiguredOnuTableProps) {
  const columns = [
    {
      title: "PON Port",
      key: "location",
      render: (_: unknown, record: UnconfiguredOnu) =>
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
      render: (_: unknown, record: UnconfiguredOnu) => (
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
        `${record.slot}-${record.port}-${record.serialNumber}`
      }
      columns={columns}
      dataSource={dataSource}
      loading={isLoading}
      pagination={false}
      locale={{ emptyText: "No unconfigured ONUs detected" }}
    />
  );
}
