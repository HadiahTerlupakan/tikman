import { useState } from "react";
import {
  Alert,
  Card,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import type { OwnPrefix, OwnPrefixPeer } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { FlowDraftBlocks } from "./FlowDraftBlocks";
import { inboundDraft, inboundTargets } from "./ownPrefixDraft";

const PERCENT = 100;

interface Props {
  available: boolean;
  rows: OwnPrefix[];
  loading: boolean;
  failed: boolean;
}

export function FlowOwnPrefixPanel({
  available,
  rows,
  loading,
  failed,
}: Props) {
  return (
    <Card
      size="small"
      title={
        <Tooltip title="Prefix yang diumumkan router ke peer BGP. Download per peer dari NetFlow; prepend dari aturan filter OUT, dibaca setiap jam. Prepend memengaruhi lewat mana download masuk, tetapi jaringan pengirim yang memutuskan.">
          Prefix milik Anda
        </Tooltip>
      }
    >
      {!available ? (
        <Alert
          type="info"
          showIcon
          message="Data prefix sendiri disimpan 30 hari. Pilih rentang di dalam 30 hari terakhir."
        />
      ) : failed ? (
        <Alert type="error" showIcon message="Gagal memuat prefix milik Anda" />
      ) : (
        <Table<OwnPrefix>
          size="small"
          rowKey="prefix"
          loading={loading}
          dataSource={rows}
          pagination={false}
          locale={{
            emptyText:
              "Belum ada prefix yang terbaca dari router. Router dibaca setiap jam.",
          }}
          expandable={{ expandedRowRender: (r) => <OwnPrefixDetail own={r} /> }}
          columns={[
            { title: "Prefix", dataIndex: "prefix" },
            {
              title: "Download",
              dataIndex: "downloadBytes",
              render: formatBytes,
            },
            { title: "Masuk lewat", render: (_, r) => mainPeer(r) },
            {
              title: "Pengumuman ke peer",
              render: (_, r) => (
                <Space size={[4, 4]} wrap>
                  {r.peers.map((p) => (
                    <AnnounceTag key={p.peer} peer={p} />
                  ))}
                </Space>
              ),
            },
          ]}
        />
      )}
    </Card>
  );
}

function mainPeer(own: OwnPrefix): string {
  if (own.downloadBytes === 0) return "—";
  const top = own.peers.reduce((a, b) =>
    b.downloadBytes > a.downloadBytes ? b : a,
  );
  return `${top.peer} (${Math.round((top.downloadBytes / own.downloadBytes) * PERCENT)}%)`;
}

function AnnounceTag({ peer }: { peer: OwnPrefixPeer }) {
  if (!peer.rule) return <Tag>{peer.peer}: ?</Tag>;
  if (!peer.announced)
    return <Tag color="red">{peer.peer}: tidak diumumkan</Tag>;
  return (
    <Tag color={peer.prepend === 0 ? "green" : "default"}>
      {peer.peer}:{" "}
      {peer.prepend === 0 ? "tanpa prepend" : `prepend ${peer.prepend}`}
    </Tag>
  );
}

function OwnPrefixDetail({ own }: { own: OwnPrefix }) {
  const targets = inboundTargets(own);
  const [target, setTarget] = useState(targets[0]?.peer ?? "");
  return (
    <div>
      <Table<OwnPrefixPeer>
        size="small"
        rowKey="peer"
        pagination={false}
        dataSource={own.peers}
        columns={[
          {
            title: "Peer",
            render: (_, p) => (
              <>
                {p.peer}
                {p.transit && <Tag style={{ marginLeft: 6 }}>transit</Tag>}
              </>
            ),
          },
          { title: "Pengumuman", render: (_, p) => <AnnounceTag peer={p} /> },
          {
            title: "Download",
            dataIndex: "downloadBytes",
            render: formatBytes,
          },
          { title: "Upload", dataIndex: "uploadBytes", render: formatBytes },
        ]}
      />
      {targets.length > 0 && (
        <div style={{ marginTop: 12 }}>
          <Space style={{ marginBottom: 8 }}>
            <Typography.Text strong>
              Arahkan download untuk {own.prefix} lewat
            </Typography.Text>
            <Select
              size="small"
              value={target}
              onChange={setTarget}
              style={{ minWidth: 180 }}
              options={targets.map((p) => ({ value: p.peer, label: p.peer }))}
            />
          </Space>
          <FlowDraftBlocks draft={inboundDraft(own, target)} />
        </div>
      )}
    </div>
  );
}
