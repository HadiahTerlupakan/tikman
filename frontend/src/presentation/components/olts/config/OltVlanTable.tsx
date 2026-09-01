import { Empty, Space, Table, Tag, Typography } from "antd";
import type { OltVlan, OltVlanPort } from "@/domain/entities";

const { Text } = Typography;

function PortTags({ ports }: { ports: OltVlanPort[] }) {
  if (ports.length === 0) {
    return <Text type="secondary">not reported</Text>;
  }

  return (
    <Space size={4} wrap>
      {ports.map((port) => (
        <Tag
          key={`${port.slot}/${port.port}`}
          color={port.tagged ? "blue" : "green"}
        >
          1/{port.slot}/{port.port}
        </Tag>
      ))}
    </Space>
  );
}

interface OltVlanTableProps {
  vlans: OltVlan[];
}

// Tagged and untagged are split into their own columns because that is the
// distinction an operator is checking: a VLAN carried tagged on an uplink is a
// trunk member, one carried untagged is an access port.
export function OltVlanTable({ vlans }: OltVlanTableProps) {
  if (vlans.length === 0) {
    return <Empty description="No VLANs read from the OLT yet" />;
  }

  return (
    <Table<OltVlan>
      size="small"
      scroll={{ x: 640 }}
      rowKey="vlanId"
      dataSource={vlans}
      pagination={false}
      columns={[
        { title: "VLAN ID", dataIndex: "vlanId", width: 100 },
        { title: "Name", dataIndex: "name", width: 180 },
        {
          title: "Tagged ports",
          render: (_, vlan) => (
            <PortTags ports={(vlan.ports ?? []).filter((p) => p.tagged)} />
          ),
        },
        {
          title: "Untagged ports",
          render: (_, vlan) => (
            <PortTags ports={(vlan.ports ?? []).filter((p) => !p.tagged)} />
          ),
        },
      ]}
    />
  );
}
