import { useState } from "react";
import {
  Badge,
  Button,
  Card,
  Popconfirm,
  Space,
  Table,
  Tooltip,
  Typography,
  message,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  useDeleteWireguardPeer,
  useUpdateWireguardPeer,
  useWireguardPeers,
} from "@/application/hooks";
import type { WireguardPeer } from "@/domain/entities";
import { formatBytes } from "../components/trafficFormat";
import { PageHeader } from "../components/common/PageHeader";
import { VpnConfigModal } from "./vpn/VpnConfigModal";
import { VpnReachabilityModal } from "./vpn/VpnReachabilityModal";
import { VpnPeerFormModal } from "./vpn/VpnPeerFormModal";
import { VpnServerCard } from "./vpn/VpnServerCard";
import { describeTunnel } from "./vpn/vpnStatus";

export default function VpnPage() {
  const [formOpen, setFormOpen] = useState(false);
  const [editingPeer, setEditingPeer] = useState<WireguardPeer | null>(null);
  const [configPeerId, setConfigPeerId] = useState<string | null>(null);
  const [testPeer, setTestPeer] = useState<WireguardPeer | null>(null);
  const { data: peers, isLoading } = useWireguardPeers();
  const updatePeer = useUpdateWireguardPeer();
  const deletePeer = useDeleteWireguardPeer();

  const columns = [
    { title: "Tunnel", dataIndex: "name", key: "name" },
    {
      title: "Alamat tunnel",
      dataIndex: "tunnelAddress",
      key: "tunnelAddress",
    },
    {
      title: "Subnet site",
      dataIndex: "allowedIps",
      key: "allowedIps",
      render: (allowedIps: string[]) => allowedIps.join(", "),
    },
    {
      title: "Status",
      key: "status",
      render: (_: unknown, peer: WireguardPeer) => {
        const described = describeTunnel(peer, new Date());
        return (
          <Tooltip title={described.hint}>
            <div>
              <Badge status={described.tone} text={described.label} />
              {described.detail && (
                <div>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {described.detail}
                  </Typography.Text>
                </div>
              )}
            </div>
          </Tooltip>
        );
      },
    },
    {
      title: "Trafik",
      key: "traffic",
      render: (_: unknown, peer: WireguardPeer) => (
        // Counters are what separate "the tunnel came up" from "data is
        // actually crossing it" — a handshake alone proves only the former.
        <Tooltip title="Diterima dari site / dikirim ke site sejak tunnel dibuat">
          <Typography.Text style={{ fontSize: 12 }}>
            &darr; {formatBytes(peer.rxBytes)} &nbsp; &uarr;{" "}
            {formatBytes(peer.txBytes)}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: "Aksi",
      key: "actions",
      render: (_: unknown, peer: WireguardPeer) => (
        <Space>
          <Button
            size="small"
            onClick={() => {
              setEditingPeer(peer);
              setFormOpen(true);
            }}
          >
            Sunting
          </Button>
          <Button size="small" onClick={() => setConfigPeerId(peer.id)}>
            Konfigurasi
          </Button>
          <Button size="small" onClick={() => setTestPeer(peer)}>
            Uji koneksi
          </Button>
          <Button
            size="small"
            onClick={() =>
              updatePeer.mutate(
                { id: peer.id, data: { enabled: !peer.enabled } },
                {
                  onError: () => message.error("Gagal mengubah status tunnel"),
                },
              )
            }
          >
            {peer.enabled ? "Nonaktifkan" : "Aktifkan"}
          </Button>
          <Popconfirm
            title="Hapus tunnel site ini?"
            okText="Hapus"
            cancelText="Batal"
            onConfirm={() =>
              deletePeer.mutate(peer.id, {
                onError: () => message.error("Gagal menghapus tunnel"),
              })
            }
          >
            <Button size="small" danger>
              Hapus
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <PageHeader
        title="VPN"
        description="Akses site yang tidak punya IP publik"
      />
      <VpnServerCard />
      <Card
        title="Site terhubung"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditingPeer(null);
              setFormOpen(true);
            }}
          >
            Tambah tunnel
          </Button>
        }
      >
        <Table
          rowKey="id"
          scroll={{ x: 900 }}
          loading={isLoading}
          dataSource={peers}
          columns={columns}
          pagination={false}
        />
      </Card>
      <VpnPeerFormModal
        open={formOpen}
        peer={editingPeer}
        onClose={() => {
          setFormOpen(false);
          setEditingPeer(null);
        }}
      />
      <VpnConfigModal
        peerId={configPeerId}
        onClose={() => setConfigPeerId(null)}
      />
      <VpnReachabilityModal
        peerId={testPeer?.id ?? null}
        siteName={testPeer?.name ?? ""}
        subnets={testPeer?.allowedIps ?? []}
        onClose={() => setTestPeer(null)}
      />
    </Space>
  );
}
