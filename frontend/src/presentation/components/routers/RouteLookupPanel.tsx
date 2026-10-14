import { useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Input,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import type { RouteCandidate } from "@/domain/entities";
import { UserRole } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useAsNames, useRouteLookup } from "@/application/hooks/useRouterPaths";
import { ApiError } from "@/infrastructure/http";
import { AsPathDiagram } from "./AsPathDiagram";
import { buildPathGraph } from "./asPathGraph";
import { graphAsns, lookupPaths } from "./bgpPaths";

const OPERATORS_ONLY =
  "Hanya admin dan teknisi yang bisa menanyakan router secara langsung; setiap kueri memindai seluruh tabel BGP router.";
const PLACEHOLDER = "IP atau prefix, mis. 103.10.0.1 atau 103.10.0.0/24";

function lookupError(error: unknown): string {
  if (error instanceof ApiError && error.code === "LOOKUP_BUSY") {
    return "Pencarian lain sedang berjalan di router ini. Coba lagi sebentar.";
  }
  if (error instanceof ApiError && error.code === "LOOKUP_TARGET") {
    return "Masukkan alamat IPv4 atau prefix antara /8 dan /32.";
  }
  return "Router tidak menjawab. Periksa status API di tab Router.";
}

const columns = [
  { title: "Sesi", dataIndex: "session" },
  {
    title: "AS path",
    dataIndex: "asPath",
    render: (p: string) => (
      <Typography.Text code>{p || "(lokal)"}</Typography.Text>
    ),
  },
  { title: "Local-pref", dataIndex: "localPref" },
  { title: "Gateway", dataIndex: "gateway" },
  {
    title: "Dipilih",
    dataIndex: "active",
    render: (active: boolean) =>
      active ? <Tag color="green">Aktif</Tag> : <Tag>Alternatif</Tag>,
  },
];

interface Props {
  routerId: string;
  routerName: string;
}

export function RouteLookupPanel({ routerId, routerName }: Props) {
  const role = useAuthStore((state) => state.user?.role);
  const canOperate = role === UserRole.ADMIN || role === UserRole.TECHNICIAN;
  const [target, setTarget] = useState("");
  const lookup = useRouteLookup(routerId);
  const result = lookup.data;
  const graph = useMemo(
    () => buildPathGraph(result ? lookupPaths(result) : []),
    [result],
  );
  const names = useAsNames(routerId, graphAsns(graph));
  const active = result?.routes.find((r) => r.active);

  return (
    <Card
      size="small"
      title="Cek jalur ke IP atau prefix"
      extra={
        <Typography.Text type="secondary">
          Ditanyakan langsung ke {routerName}, sekitar 1-2 detik per kueri
        </Typography.Text>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        {canOperate ? (
          <Space.Compact style={{ width: "100%", maxWidth: 520 }}>
            <Input
              placeholder={PLACEHOLDER}
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              onPressEnter={() => lookup.mutate(target.trim())}
              disabled={lookup.isPending}
            />
            <Button
              type="primary"
              loading={lookup.isPending}
              disabled={target.trim() === ""}
              onClick={() => lookup.mutate(target.trim())}
            >
              Cek di router
            </Button>
          </Space.Compact>
        ) : (
          <Alert type="info" showIcon message={OPERATORS_ONLY} />
        )}
        {lookup.isError && (
          <Alert type="error" showIcon message={lookupError(lookup.error)} />
        )}
        {result && (
          <div data-testid="lookup-result">
            {result.prefix === "" ? (
              <Alert
                type="warning"
                showIcon
                message={`Router tidak punya rute BGP untuk ${result.target} (${result.queries} kueri, sampai /8).`}
              />
            ) : (
              <Space
                direction="vertical"
                size="middle"
                style={{ width: "100%" }}
              >
                <Alert
                  type="success"
                  showIcon
                  message={`${result.target} ada di prefix ${result.prefix}: ${result.routes.length} kandidat, keluar lewat ${active?.session ?? "-"} (${result.queries} kueri).`}
                />
                <AsPathDiagram
                  graph={graph}
                  names={names.data ?? {}}
                  routerLabel={routerName}
                />
                <Table<RouteCandidate>
                  size="small"
                  rowKey={(r) => `${r.session}|${r.asPath}|${r.gateway}`}
                  columns={columns}
                  dataSource={result.routes}
                  pagination={false}
                />
              </Space>
            )}
          </div>
        )}
      </Space>
    </Card>
  );
}
