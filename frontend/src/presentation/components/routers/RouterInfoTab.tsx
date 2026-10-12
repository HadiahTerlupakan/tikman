import { useState } from "react";
import { Alert, Button, Descriptions, Popconfirm, Space, message } from "antd";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import { UserRole, type MikrotikRouter } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import {
  useDeleteMikrotikRouter,
  useRefreshRouter,
  useTrustRouterCertificate,
} from "@/application/hooks/useMikrotikRouters";
import { RouterFormModal } from "./RouterFormModal";
import { PROTOCOL_LABEL } from "./routerProtocol";
import { RouterStatusTag } from "./RouterStatusTag";
import { formatUptime, memoryUsedPercent } from "./routerTraffic";

export function RouterInfoTab({ router }: { router: MikrotikRouter }) {
  const role = useAuthStore((state) => state.user?.role);
  const isAdmin = role === UserRole.ADMIN;
  const canOperate = isAdmin || role === UserRole.TECHNICIAN;
  const [editing, setEditing] = useState(false);
  const refresh = useRefreshRouter();
  const trust = useTrustRouterCertificate();
  const remove = useDeleteMikrotikRouter();
  const navigate = useNavigate();
  const memory = memoryUsedPercent(router.memoryTotal, router.memoryFree);
  const presented =
    router.apiStatus === "certificate_changed"
      ? router.presentedFingerprint
      : null;

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {presented && (
        <Alert
          type="warning"
          message="Sertifikat API router berubah. Polling BGP berhenti sampai sertifikat baru diterima."
          description={`Fingerprint baru: ${presented}. Cocokkan dengan /certificate print di router sebelum menerima.`}
          action={
            isAdmin && (
              <Popconfirm
                title="Terima sertifikat baru?"
                onConfirm={() =>
                  trust.mutate({ id: router.id, fingerprint: presented })
                }
              >
                <Button size="small">Terima</Button>
              </Popconfirm>
            )
          }
        />
      )}
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="Status">
          <RouterStatusTag router={router} />
        </Descriptions.Item>
        <Descriptions.Item label="Host">{router.host}</Descriptions.Item>
        <Descriptions.Item label="Board">
          {router.boardName ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label="RouterOS">
          {router.rosVersion ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label="CPU">
          {router.cpuLoad === null ? "—" : `${router.cpuLoad}%`}
        </Descriptions.Item>
        <Descriptions.Item label="RAM terpakai">
          {memory === null ? "—" : `${memory}%`}
        </Descriptions.Item>
        <Descriptions.Item label="Uptime">
          {formatUptime(router.uptimeSeconds)}
        </Descriptions.Item>
        <Descriptions.Item label="SNMP terakhir terbaca">
          {router.lastSeenAt
            ? dayjs(router.lastSeenAt).format("DD MMM YYYY HH:mm:ss")
            : "—"}
        </Descriptions.Item>
        <Descriptions.Item label="Protokol">
          {PROTOCOL_LABEL[router.apiProtocol]} · port {router.apiPort}
        </Descriptions.Item>
        <Descriptions.Item label="Error API terakhir">
          {router.apiError ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label="Fingerprint sertifikat">
          {router.tlsFingerprint}
        </Descriptions.Item>
      </Descriptions>
      {canOperate && (
        <Space>
          <Button
            loading={refresh.isPending}
            onClick={() =>
              refresh.mutate(router.id, {
                onSuccess: () =>
                  message.success(
                    "Discovery dijadwalkan, hasilnya muncul dalam semenit",
                  ),
              })
            }
          >
            Refresh discovery
          </Button>
          {isAdmin && (
            <>
              <Button onClick={() => setEditing(true)}>Ubah</Button>
              <Popconfirm
                title="Hapus router ini beserta interface dan riwayat BGP-nya?"
                onConfirm={() =>
                  remove.mutate(router.id, {
                    onSuccess: () => navigate("/routers"),
                  })
                }
              >
                <Button danger>Hapus</Button>
              </Popconfirm>
            </>
          )}
        </Space>
      )}
      {editing && (
        <RouterFormModal router={router} onClose={() => setEditing(false)} />
      )}
    </Space>
  );
}
