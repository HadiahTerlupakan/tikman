import { Card, Space, Table, Tag } from "antd";
import { useMemo } from "react";
import {
  useSwitchInterfaces,
  useSwitchVlans,
} from "@/application/hooks/useSwitches";
import { matrixPorts, vlanMembership } from "./switchVlans";

const MATRIX_CELL: React.CSSProperties = {
  textAlign: "center",
  padding: "2px 6px",
  fontFamily: "monospace",
};

export function SwitchVlanTab({ switchId }: { switchId: string }) {
  const { data: vlans = [], isLoading } = useSwitchVlans(switchId);
  const { data: allPorts = [] } = useSwitchInterfaces(switchId);
  const ports = useMemo(() => matrixPorts(allPorts), [allPorts]);

  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      <Card size="small" title="Daftar VLAN">
        <Table
          size="small"
          rowKey="vid"
          loading={isLoading}
          pagination={false}
          dataSource={vlans}
          columns={[
            { title: "VLAN", dataIndex: "vid" },
            { title: "Nama", dataIndex: "name" },
          ]}
        />
      </Card>
      <Card
        size="small"
        title="Matriks VLAN × port"
        extra={
          <Space size={4}>
            <Tag color="blue">T tagged</Tag>
            <Tag color="green">U untagged</Tag>
          </Space>
        }
      >
        <div style={{ overflowX: "auto" }}>
          <table
            data-testid="vlan-matrix"
            style={{ borderCollapse: "collapse" }}
          >
            <thead>
              <tr>
                <th style={MATRIX_CELL}>VLAN</th>
                {ports.map((p) => (
                  <th key={p.name} style={MATRIX_CELL}>
                    {p.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {vlans.map((v) => (
                <tr key={v.vid}>
                  <th style={MATRIX_CELL}>{v.vid}</th>
                  {ports.map((p) => (
                    <td
                      key={p.name}
                      data-testid={`cell-${v.vid}-${p.name}`}
                      style={MATRIX_CELL}
                    >
                      {vlanMembership(p, v.vid)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </Space>
  );
}
