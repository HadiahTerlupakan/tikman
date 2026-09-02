import { useEffect, useState } from "react";
import { Alert, Button, Descriptions, Select, Space } from "antd";
import {
  useAssignOntToOdp,
  useOdpSubscribers,
  useOdps,
} from "@/application/hooks";
import type { Ont } from "@/domain/entities";
import { freePorts } from "./odpPorts";

interface OntOdpPanelProps {
  ont: Ont;
}

/**
 * Records which distribution box a subscriber's drop lands in.
 *
 * The port list offers only what is free, plus the port this subscriber already
 * holds: sending a taken port and letting the server refuse it wastes the trip
 * and says nothing about where the subscriber can actually go.
 */
export function OntOdpPanel({ ont }: OntOdpPanelProps) {
  const { data: odps } = useOdps();
  const [odpId, setOdpId] = useState<string | undefined>(ont.odpId);
  const [port, setPort] = useState<number | undefined>(ont.odpPort);
  const { data: subscribers } = useOdpSubscribers(odpId);
  const assign = useAssignOntToOdp();

  // A different box has different ports; keeping the old number would offer a
  // port that may not exist there.
  useEffect(() => {
    if (odpId !== ont.odpId) {
      setPort(undefined);
    }
  }, [odpId, ont.odpId]);

  const chosen = (odps ?? []).find((odp) => odp.id === odpId);
  const taken = (subscribers ?? [])
    .filter((other) => other.id !== ont.id)
    .map((other) => other.odpPort)
    .filter((value): value is number => typeof value === "number");
  const available = chosen
    ? freePorts(chosen.portCount, taken, ont.odpPort)
    : [];

  const current = (odps ?? []).find((odp) => odp.id === ont.odpId);

  return (
    <Space direction="vertical" style={{ width: "100%" }} size={16}>
      <Descriptions bordered column={1} size="small">
        <Descriptions.Item label="ODP saat ini">
          {current
            ? `${current.name} · port ${ont.odpPort}`
            : "Belum ditautkan"}
        </Descriptions.Item>
      </Descriptions>

      {assign.isError && (
        <Alert
          type="error"
          showIcon
          message="Gagal menautkan"
          description={(assign.error as Error).message}
        />
      )}

      <Space wrap>
        <Select
          style={{ minWidth: 220, maxWidth: "60vw" }}
          placeholder="Pilih ODP"
          value={odpId}
          onChange={setOdpId}
          options={(odps ?? []).map((odp) => ({
            value: odp.id,
            label: `${odp.name} (${odp.usedPorts}/${odp.portCount})`,
          }))}
        />
        <Select
          style={{ minWidth: 120, maxWidth: "60vw" }}
          placeholder="Port"
          value={port}
          onChange={setPort}
          disabled={!chosen}
          options={available.map((value) => ({
            value,
            label: `Port ${value}`,
          }))}
        />
        <Button
          type="primary"
          loading={assign.isPending}
          disabled={!odpId || !port}
          onClick={() =>
            assign.mutate({
              ontId: ont.id,
              odpId: odpId as string,
              port: port as number,
            })
          }
        >
          Simpan
        </Button>
      </Space>

      {chosen && available.length === 0 && (
        <Alert
          type="warning"
          showIcon
          message={`${chosen.name} sudah penuh — tidak ada port kosong`}
        />
      )}
    </Space>
  );
}
