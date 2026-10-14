import { useState } from "react";
import {
  Alert,
  Card,
  Descriptions,
  Segmented,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import type { RouteAdvice, RoutePath } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import {
  adviceText,
  mainInbound,
  needsAction,
  ROUTE_STATUS,
} from "./routeAdvice";

const PAGE_SIZE = 10;
const PERCENT = 100;

interface Props {
  available: boolean;
  rows: RouteAdvice[];
  loading: boolean;
  failed: boolean;
}

export function FlowRouteAdvicePanel({
  available,
  rows,
  loading,
  failed,
}: Props) {
  const [onlyAction, setOnlyAction] = useState(true);
  const shown = onlyAction ? rows.filter(needsAction) : rows;
  return (
    <Card
      size="small"
      title={
        <Tooltip title="Dibaca dari tabel BGP router setiap jam untuk prefix tujuan tersibuk. Local-pref hanya mengatur trafik keluar; trafik masuk ditentukan jaringan tujuan. Draf konfigurasi tidak pernah dijalankan otomatis.">
          Saran routing
        </Tooltip>
      }
      extra={
        <Segmented
          size="small"
          value={onlyAction ? "action" : "all"}
          onChange={(v) => setOnlyAction(v === "action")}
          options={[
            { label: "Perlu tindakan", value: "action" },
            { label: "Semua", value: "all" },
          ]}
        />
      }
    >
      {!available ? (
        <Alert
          type="info"
          showIcon
          message="Saran routing memakai data 7 hari terakhir. Pilih rentang di dalam 7 hari terakhir."
        />
      ) : failed ? (
        <Alert type="error" showIcon message="Gagal memuat saran routing" />
      ) : (
        <Table<RouteAdvice>
          size="small"
          rowKey="prefix"
          loading={loading}
          dataSource={shown}
          pagination={{ defaultPageSize: PAGE_SIZE }}
          locale={{
            emptyText:
              rows.length === 0
                ? "Belum ada rute yang dibaca. Router dibaca setiap jam."
                : "Tidak ada yang perlu ditindaklanjuti.",
          }}
          expandable={{ expandedRowRender: (r) => <AdviceDetail advice={r} /> }}
          columns={[
            {
              title: "Prefix",
              render: (_, r) => (
                <>
                  <div>{r.prefix}</div>
                  <Typography.Text type="secondary">
                    {r.name || `AS${r.asn}`}
                  </Typography.Text>
                </>
              ),
            },
            {
              title: "Status",
              render: (_, r) => (
                <Tag color={ROUTE_STATUS[r.status].color}>
                  {ROUTE_STATUS[r.status].label}
                </Tag>
              ),
            },
            {
              title: "Download",
              dataIndex: "downloadBytes",
              render: formatBytes,
            },
            { title: "Upload", dataIndex: "uploadBytes", render: formatBytes },
            {
              title: "Keluar lewat",
              render: (_, r) => <PathLabel path={r.active} />,
            },
            {
              title: "Masuk lewat",
              render: (_, r) => <InboundLabel advice={r} />,
            },
          ]}
        />
      )}
    </Card>
  );
}

function PathLabel({ path }: { path: RoutePath | null }) {
  if (!path) return <>—</>;
  return (
    <span>
      {path.peer}
      {path.transit && <Tag style={{ marginLeft: 6 }}>transit</Tag>}
      <Typography.Text type="secondary">
        {" "}
        · {path.pathLength} AS · LP {path.localPref}
      </Typography.Text>
    </span>
  );
}

function InboundLabel({ advice }: { advice: RouteAdvice }) {
  const main = mainInbound(advice.inbound);
  if (!main) return <>—</>;
  return (
    <span>
      {main.peer} ({Math.round(main.share * PERCENT)}%)
    </span>
  );
}

function AdviceDetail({ advice }: { advice: RouteAdvice }) {
  return (
    <div>
      <Typography.Paragraph>{adviceText(advice)}</Typography.Paragraph>
      <Descriptions size="small" column={1} bordered>
        <Descriptions.Item label="Rute aktif">
          <PathLabel path={advice.active} />
          {advice.active && (
            <Typography.Text type="secondary">
              {" "}
              — {advice.active.asPath}
            </Typography.Text>
          )}
        </Descriptions.Item>
        <Descriptions.Item label="Alternatif">
          {(advice.alternatives ?? []).length === 0
            ? "Tidak ada"
            : (advice.alternatives ?? []).map((p) => (
                <div key={p.peer}>
                  <PathLabel path={p} />
                  <Typography.Text type="secondary">
                    {" "}
                    — {p.asPath}
                  </Typography.Text>
                </div>
              ))}
        </Descriptions.Item>
        <Descriptions.Item label="Download masuk">
          {(advice.inbound ?? []).length === 0
            ? "—"
            : (advice.inbound ?? []).map((i) => (
                <div key={i.peer}>
                  {i.peer}: {formatBytes(i.bytes)}
                </div>
              ))}
        </Descriptions.Item>
      </Descriptions>
      {advice.draft && (
        <>
          <Typography.Paragraph
            style={{ marginTop: 12, marginBottom: 4 }}
            strong
          >
            Draf konfigurasi (tinjau, lalu jalankan sendiri di router)
          </Typography.Paragraph>
          <Typography.Paragraph copyable={{ text: advice.draft }}>
            <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>
              {advice.draft}
            </pre>
          </Typography.Paragraph>
        </>
      )}
    </div>
  );
}
