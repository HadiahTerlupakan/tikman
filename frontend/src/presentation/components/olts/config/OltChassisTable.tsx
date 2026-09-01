import { Empty, Table, Typography } from "antd";
import type { ChassisEntity } from "@/domain/entities";

const { Text } = Typography;

interface OltChassisTableProps {
  entities: ChassisEntity[];
}

// ENTITY-MIB numbers its entries by the OLT's own scheme, which is not the slot
// number, so the index is shown as reported rather than translated into an
// address it does not mean.
export function OltChassisTable({ entities }: OltChassisTableProps) {
  if (entities.length === 0) {
    return (
      <Empty description="The discovery poll has not read the chassis yet" />
    );
  }

  return (
    <Table
      size="small"
      scroll={{ x: 640 }}
      rowKey="index"
      dataSource={entities}
      pagination={false}
      columns={[
        { title: "Entity", dataIndex: "index", width: 90 },
        { title: "Description", dataIndex: "description" },
        {
          title: "Serial",
          dataIndex: "serial",
          render: (serial?: string) =>
            serial || <Text type="secondary">—</Text>,
        },
        {
          title: "Software",
          dataIndex: "software",
          render: (software?: string) =>
            software || <Text type="secondary">—</Text>,
        },
      ]}
    />
  );
}
