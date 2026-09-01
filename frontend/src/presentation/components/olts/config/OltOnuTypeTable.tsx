import { Empty, Table, Tag, Typography } from "antd";
import type { ZteOnuType } from "@/domain/entities";

const { Text } = Typography;

interface OltOnuTypeTableProps {
  types: ZteOnuType[];
}

// The description is the OLT's own free text ("4eth,2port,4wifi") and is shown
// verbatim: nothing on the device enforces its shape, so parsing it into port
// counts would invent precision the source does not have.
export function OltOnuTypeTable({ types }: OltOnuTypeTableProps) {
  if (types.length === 0) {
    return <Empty description="No ONU types read from the OLT yet" />;
  }

  return (
    <Table<ZteOnuType>
      size="small"
      scroll={{ x: 700 }}
      rowKey="name"
      dataSource={types}
      pagination={false}
      columns={[
        { title: "ONU type", dataIndex: "name", width: 200 },
        {
          title: "PON",
          dataIndex: "pon",
          width: 90,
          render: (pon: string) => <Tag>{pon.toUpperCase()}</Tag>,
        },
        {
          title: "Hardware",
          dataIndex: "description",
          render: (description?: string) =>
            description || <Text type="secondary">—</Text>,
        },
        {
          title: "Max T-CONT",
          dataIndex: "maxTcont",
          width: 120,
          render: (value?: number) => value || <Text type="secondary">—</Text>,
        },
        {
          title: "Max GEM port",
          dataIndex: "maxGemport",
          width: 130,
          render: (value?: number) => value || <Text type="secondary">—</Text>,
        },
      ]}
    />
  );
}
