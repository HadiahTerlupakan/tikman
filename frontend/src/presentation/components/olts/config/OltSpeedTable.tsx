import { Empty, Table, Typography } from "antd";
import type { TcontProfile } from "@/domain/entities";
import { formatBandwidth } from "./oltSystem";

const { Text } = Typography;

interface OltSpeedTableProps {
  profiles: TcontProfile[];
}

export function OltSpeedTable({ profiles }: OltSpeedTableProps) {
  if (profiles.length === 0) {
    return <Empty description="No T-CONT profiles read from the OLT yet" />;
  }

  return (
    <Table<TcontProfile>
      size="small"
      rowKey="name"
      dataSource={profiles}
      pagination={false}
      columns={[
        { title: "Profile", dataIndex: "name", width: 160 },
        { title: "T-CONT type", dataIndex: "type", width: 120 },
        {
          title: "Fixed",
          dataIndex: "fixedBwKbps",
          render: (kbps: number) => formatBandwidth(kbps),
        },
        {
          title: "Assured",
          dataIndex: "assuredBwKbps",
          render: (kbps: number) => formatBandwidth(kbps),
        },
        {
          title: "Maximum",
          dataIndex: "maxBwKbps",
          render: (kbps: number) => <Text strong>{formatBandwidth(kbps)}</Text>,
        },
      ]}
    />
  );
}
