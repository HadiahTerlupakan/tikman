import type { ReactNode } from "react";
import { Tag, Typography, theme } from "antd";
import type { SwitchInterface } from "@/domain/entities";
import { formatSpeed } from "../trafficFormat";
import { STATE_TEXT, type PortState } from "../routers/faceplate/portModel";
import { formatBps } from "../routers/routerTraffic";
import { portState, stpText } from "./switchPorts";
import { cleanLabel } from "./vlanMatrix";
import "./portDetail.css";

const STATE_COLOR: Record<PortState, string> = {
  up: "green",
  down: "red",
  silent: "orange",
  disabled: "default",
};

const count = (value: number | null) =>
  value === null ? "—" : value.toLocaleString("id-ID");

function Stat({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="pd-stat">
      <Typography.Text type="secondary" className="pd-stat-label">
        {label}
      </Typography.Text>
      <div className="pd-stat-value">{children}</div>
    </div>
  );
}

/** The allowed list split on commas, so each VLAN or range wraps as one tag. */
function VlanTags({ list }: { list: string }) {
  const items = list
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  if (items.length === 0)
    return <Typography.Text type="secondary">—</Typography.Text>;
  return (
    <div className="pd-vlans">
      {items.map((item) => (
        <Tag key={item} bordered={false} className="pd-vlan">
          {item}
        </Tag>
      ))}
    </div>
  );
}

export function SwitchPortDetail({ port }: { port: SwitchInterface }) {
  const { token } = theme.useToken();
  const state = portState(port);
  const alias = cleanLabel(port.alias);
  const isAccess = port.mode === "access";
  return (
    <div
      className="pd"
      style={{ ["--pd-border" as string]: token.colorBorderSecondary }}
    >
      <div className="pd-head">
        <Typography.Title level={5} className="pd-name">
          {port.name}
        </Typography.Title>
        <Tag color={STATE_COLOR[state]}>{STATE_TEXT[state]}</Tag>
        {port.speedMbps ? (
          <Typography.Text type="secondary">
            {formatSpeed(port.speedMbps)}
          </Typography.Text>
        ) : null}
      </div>
      {alias && <Typography.Text className="pd-alias">{alias}</Typography.Text>}

      <div className="pd-grid">
        <Stat label="Masuk (rx)">{formatBps(port.lastInBps)}</Stat>
        <Stat label="Keluar (tx)">{formatBps(port.lastOutBps)}</Stat>
        <Stat label="Error (rx / tx)">
          {`${count(port.inErrors)} / ${count(port.outErrors)}`}
        </Stat>
        <Stat label="Buang masuk">{count(port.inDiscards)}</Stat>
      </div>

      <div className="pd-section">
        <div className="pd-row">
          <Typography.Text type="secondary">Mode</Typography.Text>
          <span>{port.mode || "—"}</span>
          <Typography.Text type="secondary">PVID</Typography.Text>
          <span>{port.pvid ?? "—"}</span>
          <Typography.Text type="secondary">STP edge</Typography.Text>
          <span>{stpText(port.stpEdge)}</span>
        </div>
        {!isAccess && port.mode !== "" && (
          <>
            <Typography.Text type="secondary" className="pd-sub">
              VLAN yang dilewatkan
            </Typography.Text>
            <VlanTags list={port.allowedVlans} />
          </>
        )}
      </div>
    </div>
  );
}
