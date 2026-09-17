import { useEffect, useState } from "react";
import { Alert, Button, Descriptions, InputNumber, Select, Space } from "antd";
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
  const takenBy = new Map(
    (subscribers ?? [])
      .filter((other) => other.id !== ont.id && other.odpPort)
      .map((other) => [other.odpPort as number, other.serialNumber]),
  );
  // Capacity 0 means unlimited (see OdpPortFields, which solved this same
  // gap): freePorts(0, …) loops zero times, which read as "no ports free" for
  // a box the backend will happily accept port 999 on.
  const isUnlimited = chosen ? chosen.portCount <= 0 : false;
  const available =
    chosen && !isUnlimited
      ? freePorts(chosen.portCount, Array.from(takenBy.keys()), ont.odpPort)
      : [];

  const current = (odps ?? []).find((odp) => odp.id === ont.odpId);

  return (
    <Space direction="vertical" style={{ width: "100%" }} size={16}>
      <Descriptions bordered column={1} size="small">
        <Descriptions.Item label="ODP saat ini">
          {current
            ? `${current.code} · port ${ont.odpPort}`
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
          // Only the box and its stated capacity — never usedPorts, which
          // nothing populates for a mapping node (see toOdp in
          // DistributionRepository) and would read as a real occupancy count
          // when it is always zero.
          options={(odps ?? []).map((odp) => ({
            value: odp.id,
            label:
              odp.portCount > 0
                ? `${odp.code} (kapasitas ${odp.portCount})`
                : odp.code,
          }))}
        />
        {isUnlimited ? (
          <InputNumber
            style={{ minWidth: 120 }}
            min={1}
            placeholder="Nomor port"
            value={port}
            onChange={(value) => setPort(value ?? undefined)}
          />
        ) : (
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
        )}
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

      {isUnlimited && takenBy.size > 0 && (
        <Alert
          type="info"
          showIcon
          message={`Sudah dipakai: ${Array.from(takenBy.entries())
            .map(([p, serial]) => `${p} (${serial})`)
            .join(", ")}`}
        />
      )}

      {chosen && !isUnlimited && available.length === 0 && (
        <Alert
          type="warning"
          showIcon
          message={`${chosen.code} sudah penuh — tidak ada port kosong`}
        />
      )}
    </Space>
  );
}
