import { Alert, Card, Col, Progress, Row, Table, Tag, Typography } from "antd";
import type { CSSProperties } from "react";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { RouterBgpSession, TrafficRange } from "@/domain/entities";
import { useInterfaceTraffic } from "@/application/hooks/useMikrotikRouters";
import { formatSpeed } from "../../trafficFormat";
import { TrafficChart } from "../TrafficChart";
import { directionLabels, formatBps, interfaceSeries } from "../routerTraffic";
import {
  STATE_TEXT,
  type PortState,
  type PortView,
  type VlanView,
} from "./portModel";
// The VLAN swatch takes its colour from the faceplate's role classes.
import "./faceplate.css";

const LAST_24H: TrafficRange = { kind: "relative", hours: 24 };
const BITS_PER_MEGABIT = 1_000_000;
const FULL = 100;

const SWATCH: CSSProperties = {
  display: "inline-block",
  width: 10,
  height: 10,
  marginRight: 6,
  borderRadius: 1,
  background: "var(--fp-role)",
  boxShadow: "inset 0 0 0 1px rgba(0, 0, 0, 0.15)",
};
const STATE_LINE: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 6,
  margin: "10px 0",
  fontWeight: 600,
};
const FIGURE: CSSProperties = {
  font: "600 20px/1.2 var(--fp-condensed)",
  fontVariantNumeric: "tabular-nums",
};

const STATE_DOT: Record<PortState, string> = {
  up: "var(--fp-led-up)",
  down: "var(--fp-led-down)",
  silent: "var(--fp-led-warn)",
  disabled: "var(--fp-led-off)",
};

interface SessionRow {
  session: RouterBgpSession;
  via: string;
}

const bgpColumns: ColumnsType<SessionRow> = [
  { title: "Sesi", render: (_, r) => r.session.name },
  { title: "Lewat", dataIndex: "via" },
  {
    title: "Prefix",
    align: "right",
    render: (_, r) => r.session.prefixCount.toLocaleString("id-ID"),
  },
  {
    title: "Status",
    render: (_, r) =>
      r.session.state === "established" ? (
        <Tag color="green">Established</Tag>
      ) : (
        <Tag color="red">Down</Tag>
      ),
  },
];

const vlanColumns: ColumnsType<VlanView> = [
  {
    title: "VLAN",
    render: (_, v) => (
      <>
        <i className={`fp-role-${v.role}`} style={SWATCH} />
        {v.iface.vlanId}
      </>
    ),
  },
  { title: "Nama", dataIndex: "label" },
  { title: "Masuk", align: "right", render: (_, v) => formatBps(v.inBps) },
];

/** A session on a VLAN is reached through that VLAN, not the port itself. */
function sessionRows(view: PortView): SessionRow[] {
  return view.sessions.map((session) => {
    const vlan = view.vlans.find((v) => v.iface.id === session.interfaceId);
    return {
      session,
      via: vlan ? `VLAN ${vlan.iface.vlanId}` : view.iface.name,
    };
  });
}

function utilisation(view: PortView): number {
  const speed = view.iface.speedMbps;
  if (!speed) return 0;
  const peak = Math.max(view.inBps, view.outBps);
  return Math.min(FULL, (peak / (speed * BITS_PER_MEGABIT)) * FULL);
}

function StateLine({ view, offline }: { view: PortView; offline: boolean }) {
  const silentSince = view.iface.silentSince;
  return (
    <p style={STATE_LINE}>
      <i
        style={{
          width: 9,
          height: 9,
          borderRadius: "50%",
          background: STATE_DOT[offline ? "disabled" : view.state],
        }}
      />
      {offline ? "Tidak terjangkau" : STATE_TEXT[view.state]}
      {!offline && view.state === "silent" && silentSince && (
        <Typography.Text type="secondary">
          {` — link up, tapi tidak ada trafik masuk sejak ${dayjs(silentSince).format("DD MMM HH:mm")}`}
        </Typography.Text>
      )}
    </p>
  );
}

function Figures({ view, offline }: { view: PortView; offline: boolean }) {
  const speed = view.iface.speedMbps;
  const util = utilisation(view);
  const live = (text: string) => (offline ? "—" : text);
  const figures: [string, string][] = [
    ["Kecepatan link", speed ? formatSpeed(speed) : "—"],
    ["Masuk", live(formatBps(view.inBps))],
    ["Keluar", live(formatBps(view.outBps))],
    ["Terpakai", live(`${util.toFixed(0)}%`)],
  ];
  return (
    <>
      <Row gutter={12}>
        {figures.map(([term, value]) => (
          <Col key={term} xs={12} md={6}>
            <Typography.Text type="secondary">{term}</Typography.Text>
            <div style={FIGURE}>{value}</div>
          </Col>
        ))}
      </Row>
      <Progress percent={offline ? 0 : util} showInfo={false} />
    </>
  );
}

function InboundChart(props: { routerId: string; view: PortView }) {
  const { iface, roles } = props.view;
  const { data: points = [] } = useInterfaceTraffic(
    props.routerId,
    iface.id,
    LAST_24H,
  );
  const labels = directionLabels(roles.includes("upstream"));
  return (
    <TrafficChart
      title="Trafik masuk, 24 jam"
      series={[labels.in]}
      data={interfaceSeries(points, labels)}
    />
  );
}

function EmptyNote({ text }: { text: string }) {
  return <Typography.Paragraph type="secondary">{text}</Typography.Paragraph>;
}

function SessionsAndVlans({ view, stale }: { view: PortView; stale: boolean }) {
  const rows = sessionRows(view);
  return (
    <Card size="small" title="Sesi BGP">
      {stale && (
        <Alert
          type="warning"
          showIcon
          message="Data BGP usang: API router sedang gagal"
          style={{ marginBottom: 12 }}
        />
      )}
      {rows.length > 0 ? (
        <Table
          size="small"
          rowKey={(r) => r.session.id}
          pagination={false}
          columns={bgpColumns}
          dataSource={rows}
        />
      ) : (
        <EmptyNote text="Tidak ada sesi BGP lewat port ini." />
      )}
      <Typography.Title level={5} style={{ marginTop: 16 }}>
        VLAN
      </Typography.Title>
      {view.vlans.length > 0 ? (
        <Table
          size="small"
          rowKey={(v) => v.iface.id}
          pagination={false}
          columns={vlanColumns}
          dataSource={view.vlans}
        />
      ) : (
        <EmptyNote text="Tidak ada VLAN di port ini." />
      )}
      <Typography.Paragraph type="secondary" style={{ marginTop: 12 }}>
        Label port dan nama VLAN diambil dari comment di MikroTik.
      </Typography.Paragraph>
    </Card>
  );
}

interface Props {
  routerId: string;
  view: PortView;
  offline: boolean;
  stale: boolean;
}

/** What one port carries: its state, traffic, and the sessions and VLANs that run over it. */
export function PortDetailPanel({ routerId, view, offline, stale }: Props) {
  const comment = view.iface.comment.trim();
  return (
    <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
      <Col xs={24} lg={11}>
        <Card size="small">
          <Typography.Title level={4}>{view.label}</Typography.Title>
          <Typography.Text type="secondary">
            {comment
              ? `${view.iface.name} · comment di router: “${comment}”`
              : view.iface.name}
          </Typography.Text>
          <StateLine view={view} offline={offline} />
          <Figures view={view} offline={offline} />
          <InboundChart routerId={routerId} view={view} />
        </Card>
      </Col>
      <Col xs={24} lg={13}>
        <SessionsAndVlans view={view} stale={stale} />
      </Col>
    </Row>
  );
}
