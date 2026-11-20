import { Alert, Empty, Typography } from "antd";
import { useMemo, useState, type CSSProperties } from "react";
import type { ManagedSwitch, SwitchInterface } from "@/domain/entities";
import { useSwitchInterfaces } from "@/application/hooks/useSwitches";
import { layoutFaceplate } from "../routers/faceplate/portLayout";
import { PortLabels } from "../routers/faceplate/PortLabels";
import { SwitchPortDetail } from "./SwitchPortDetail";
import {
  NO_HEALTH,
  SWITCH_PORT_KINDS,
  buildSwitchPortViews,
  physicalSwitchPorts,
  switchReachable,
} from "./switchPorts";

const STAGE: CSSProperties = {
  border: "1px solid var(--fp-stage-line)",
  borderRadius: 10,
  overflowX: "auto",
};

// A stable fallback: a fresh [] per render would rebuild the views the label layout depends on.
const NO_PORTS: SwitchInterface[] = [];

/** The switch's front panel, labelled like a router's, and the detail of the port picked on it. */
export function SwitchFaceplate({ sw }: { sw: ManagedSwitch }) {
  const { data = NO_PORTS, isLoading, isError } = useSwitchInterfaces(sw.id);
  const [picked, setPicked] = useState<string>();
  const physical = useMemo(() => physicalSwitchPorts(data), [data]);
  const layout = useMemo(
    () => layoutFaceplate(sw.model, physical, SWITCH_PORT_KINDS[sw.vendor]),
    [sw.model, sw.vendor, physical],
  );
  const ports = useMemo(() => buildSwitchPortViews(data), [data]);

  if (isLoading) return null;
  if (isError) return <Alert type="error" message="Data port gagal dimuat" />;
  if (physical.length === 0) {
    return <Empty description="Belum ada data interface" />;
  }
  const port = data.find((p) => p.name === picked);
  return (
    <div style={{ marginBottom: 16 }}>
      {!layout.templated && (
        <Typography.Paragraph type="secondary">
          {`Model ${layout.model || "ini"} belum punya cetakan, jadi susunan port dibuat otomatis dari nama interface.`}
        </Typography.Paragraph>
      )}
      <div style={STAGE}>
        <PortLabels
          layout={layout}
          ports={ports}
          offline={!switchReachable(sw)}
          health={NO_HEALTH}
          selected={picked}
          onSelect={setPicked}
        />
      </div>
      {port && (
        <div style={{ marginTop: 16 }}>
          <SwitchPortDetail port={port} />
        </div>
      )}
    </div>
  );
}
