import { useMemo, useState } from "react";
import { Card, Empty, Input, Space, Switch, Typography } from "antd";
import {
  useSwitchInterfaces,
  useSwitchVlans,
} from "@/application/hooks/useSwitches";
import { SwitchVlanMatrix } from "./SwitchVlanMatrix";
import { matrixPorts } from "./switchVlans";
import { isIdlePort, portGroups, vlanLabel } from "./vlanMatrix";

export function SwitchVlanTab({ switchId }: { switchId: string }) {
  const { data: vlans = [], isLoading } = useSwitchVlans(switchId);
  const { data: allPorts = [] } = useSwitchInterfaces(switchId);
  const [query, setQuery] = useState("");
  const [showIdle, setShowIdle] = useState(false);

  const ports = useMemo(() => matrixPorts(allPorts), [allPorts]);
  const idleCount = ports.filter(isIdlePort).length;
  const groups = useMemo(
    () => portGroups(showIdle ? ports : ports.filter((p) => !isIdlePort(p))),
    [ports, showIdle],
  );
  const q = query.trim().toLowerCase();
  const shown = useMemo(
    () =>
      q
        ? vlans.filter(
            (v) =>
              String(v.vid).includes(q) ||
              vlanLabel(v).toLowerCase().includes(q),
          )
        : vlans,
    [vlans, q],
  );
  const count = q ? `${shown.length} dari ${vlans.length}` : vlans.length;

  return (
    <Card
      size="small"
      loading={isLoading}
      title={
        <Space size={8}>
          Matriks VLAN × port
          <Typography.Text type="secondary">{count} VLAN</Typography.Text>
        </Space>
      }
      extra={
        <Space size="middle" wrap>
          <Input.Search
            allowClear
            size="small"
            placeholder="Cari VLAN atau nama"
            style={{ width: 170 }}
            onChange={(e) => setQuery(e.target.value)}
          />
          <label style={{ cursor: "pointer", whiteSpace: "nowrap" }}>
            <Space size={6}>
              <Switch size="small" checked={showIdle} onChange={setShowIdle} />
              <Typography.Text type="secondary">
                Port kosong ({idleCount})
              </Typography.Text>
            </Space>
          </label>
        </Space>
      }
    >
      {shown.length === 0 ? (
        <Empty
          description={
            q ? "Tidak ada VLAN yang cocok" : "Belum ada data VLAN dari switch"
          }
        />
      ) : (
        <SwitchVlanMatrix vlans={shown} groups={groups} countPorts={ports} />
      )}
    </Card>
  );
}
