import { Col, Row, Space, Spin, Typography } from "antd";
import type { MikrotikRouter, TrafficRange } from "@/domain/entities";
import { useRouterBras } from "@/application/hooks/useMikrotikRouters";
import { useNow } from "../faceplate/useNow";
import { BrasPools } from "./BrasPools";
import { BrasRadius } from "./BrasRadius";
import { BrasSessionsChart } from "./BrasSessionsChart";
import { BrasSignals } from "./BrasSignals";
import { BrasTopUsers } from "./BrasTopUsers";
import { BrasUplink } from "./BrasUplink";
import "./bras.css";

const LAST_DAY: TrafficRange = { kind: "relative", hours: 24 };
// Staleness is judged by the clock, which moves even when no refetch lands.
const CLOCK_TICK_MS = 30_000;

export function BrasSummaryTab({ router }: { router: MikrotikRouter }) {
  const { data: view, isLoading } = useRouterBras(router.id, LAST_DAY);
  const now = useNow(CLOCK_TICK_MS);
  if (isLoading) return <Spin />;
  if (!view) {
    return (
      <Typography.Text type="danger">Data BRAS gagal dimuat</Typography.Text>
    );
  }
  const snap = view.bras;
  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <BrasSignals view={view} now={now} />
      <section>
        <div className="bras-chart-head">
          <Typography.Title level={5}>
            Pelanggan online, 24 jam terakhir
          </Typography.Title>
          <Typography.Text type="secondary">
            Titik merah: turun sedikitnya 100 sesi dan 5% dari pembacaan
            sebelumnya
          </Typography.Text>
        </div>
        <BrasSessionsChart samples={view.samples} drops={view.drops} />
      </section>
      <Row gutter={[24, 24]}>
        <Col xs={24} lg={12}>
          <Space direction="vertical" size="large" style={{ width: "100%" }}>
            <BrasPools pools={snap?.pools ?? []} />
            <BrasRadius servers={snap?.radius ?? []} />
          </Space>
        </Col>
        <Col xs={24} lg={12}>
          <Space direction="vertical" size="large" style={{ width: "100%" }}>
            <BrasTopUsers
              users={snap?.topUsers ?? []}
              queuesFound={snap?.queuesFound ?? false}
            />
            {router.uplinkInterfaceId && (
              <BrasUplink
                routerId={router.id}
                uplinkInterfaceId={router.uplinkInterfaceId}
              />
            )}
          </Space>
        </Col>
      </Row>
    </Space>
  );
}
