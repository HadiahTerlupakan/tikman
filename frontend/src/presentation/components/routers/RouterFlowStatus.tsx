import { Alert, Descriptions, Space } from "antd";
import dayjs from "dayjs";
import type { MikrotikRouter } from "@/domain/entities";
import { useFlowStatus } from "@/application/hooks/useRouterFlows";
import { flowStale } from "./routerFlows";

const DATE_FORMAT = "DD MMM YYYY HH:mm:ss";

export function RouterFlowStatus({ router }: { router: MikrotikRouter }) {
  const { data: status } = useFlowStatus(router.id);
  const dataset = status?.asnDatasetUpdatedAt
    ? `${dayjs(status.asnDatasetUpdatedAt).format(DATE_FORMAT)} · ${status.asnCount} ASN`
    : "Belum dimuat";
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {flowStale(router, Date.now()) && (
        <Alert
          type="warning"
          showIcon
          message="NetFlow aktif tetapi tidak ada flow masuk lebih dari 5 menit"
          description="Periksa /ip traffic-flow dan target-nya di router, serta firewall UDP 2055 di VPS."
        />
      )}
      <Descriptions bordered size="small" column={1} title="NetFlow">
        <Descriptions.Item label="Status">
          {router.flowEnabled ? "Aktif" : "Nonaktif"}
        </Descriptions.Item>
        <Descriptions.Item label="Rasio sampling">
          {`1:${router.flowSampling}`}
        </Descriptions.Item>
        <Descriptions.Item label="Flow terakhir diterima">
          {router.flowLastSeenAt
            ? dayjs(router.flowLastSeenAt).format(DATE_FORMAT)
            : "—"}
        </Descriptions.Item>
        <Descriptions.Item label="Dataset ASN">{dataset}</Descriptions.Item>
      </Descriptions>
    </Space>
  );
}
