import { useMemo, useState } from "react";
import { Drawer, Input, Select, Space, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { SwitchInterface } from "@/domain/entities";
import { useSwitchInterfaces } from "@/application/hooks/useSwitches";
import { STATE_TEXT, type PortState } from "../routers/faceplate/portModel";
import { formatBps } from "../routers/routerTraffic";
import { formatSpeed } from "../trafficFormat";
import { SwitchPortDetail } from "./SwitchPortDetail";
import { portState, stpText, vlanText } from "./switchPorts";
import { cleanLabel } from "./vlanMatrix";

const STATE_COLOR: Record<PortState, string> = {
  up: "green",
  down: "red",
  silent: "orange",
  disabled: "default",
};

const ALL = "";
const STATUS_OPTIONS = [
  { value: ALL, label: "Semua status" },
  { value: "up", label: STATE_TEXT.up },
  { value: "down", label: STATE_TEXT.down },
  { value: "disabled", label: STATE_TEXT.disabled },
];
const MODE_OPTIONS = [
  { value: ALL, label: "Semua mode" },
  { value: "access", label: "access" },
  { value: "trunk", label: "trunk" },
  { value: "hybrid", label: "hybrid" },
];

const columns: ColumnsType<SwitchInterface> = [
  { title: "Port", dataIndex: "name" },
  {
    title: "Status",
    render: (_, p) => (
      <Tag color={STATE_COLOR[portState(p)]}>{STATE_TEXT[portState(p)]}</Tag>
    ),
  },
  {
    title: "Kecepatan",
    render: (_, p) => (p.speedMbps ? formatSpeed(p.speedMbps) : "—"),
  },
  { title: "Mode", render: (_, p) => p.mode || "—" },
  { title: "PVID / VLAN", render: (_, p) => vlanText(p) },
  { title: "STP edge", render: (_, p) => stpText(p.stpEdge) },
  {
    title: "Trafik (rx / tx)",
    render: (_, p) => `${formatBps(p.lastInBps)} / ${formatBps(p.lastOutBps)}`,
  },
  {
    title: "Error (rx / tx)",
    render: (_, p) => `${p.inErrors ?? "—"} / ${p.outErrors ?? "—"}`,
  },
  { title: "Alias", render: (_, p) => cleanLabel(p.alias) || "—" },
];

function matches(
  p: SwitchInterface,
  search: string,
  status: string,
  mode: string,
) {
  const text = `${p.name} ${p.alias}`.toLowerCase();
  return (
    text.includes(search.toLowerCase()) &&
    (status === ALL || portState(p) === status) &&
    (mode === ALL || p.mode === mode)
  );
}

export function SwitchPortsTab({ switchId }: { switchId: string }) {
  const { data = [], isLoading } = useSwitchInterfaces(switchId);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState(ALL);
  const [mode, setMode] = useState(ALL);
  const [selected, setSelected] = useState<SwitchInterface | null>(null);
  const rows = useMemo(
    () =>
      data
        .filter((p) => matches(p, search, status, mode))
        .sort((a, b) => a.name.localeCompare(b.name, "en", { numeric: true })),
    [data, search, status, mode],
  );

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Input.Search
          allowClear
          placeholder="Cari port atau alias"
          onChange={(e) => setSearch(e.target.value)}
          style={{ width: 280 }}
        />
        <Select
          value={status}
          options={STATUS_OPTIONS}
          onChange={setStatus}
          style={{ width: 150 }}
        />
        <Select
          value={mode}
          options={MODE_OPTIONS}
          onChange={setMode}
          style={{ width: 150 }}
        />
      </Space>
      <Table
        size="small"
        rowKey="id"
        loading={isLoading}
        pagination={false}
        scroll={{ x: "max-content" }}
        columns={columns}
        dataSource={rows}
        onRow={(port) => ({
          onClick: () => setSelected(port),
          style: { cursor: "pointer" },
        })}
      />
      <Drawer
        open={selected !== null}
        width={560}
        title="Detail port"
        onClose={() => setSelected(null)}
      >
        {selected && <SwitchPortDetail port={selected} />}
      </Drawer>
    </Space>
  );
}
