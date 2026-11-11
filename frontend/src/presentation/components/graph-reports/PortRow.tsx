import { Form, Input, Select } from "antd";
import { useLibrenmsPorts } from "@/application/hooks";
import type { LibrenmsDevice } from "@/domain/entities";
import {
  defaultPortLabel,
  deviceDisplayName,
  deviceOptions,
  portOptionLabel,
  type PortFormValue,
} from "./graphReportForm";
import type { PreviewTarget } from "./PortPreviewModal";
import { PortRowActions } from "./PortRowActions";

interface PortRowProps {
  name: number;
  index: number;
  count: number;
  devices: LibrenmsDevice[];
  devicesLoading: boolean;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
  onPreview: (target: PreviewTarget) => void;
}

const FIELD_STYLE = { flex: "1 1 200px", marginBottom: 8 };

export function PortRow({
  name,
  index,
  count,
  devices,
  devicesLoading,
  onMove,
  onRemove,
  onPreview,
}: PortRowProps) {
  const form = Form.useFormInstance();
  const value: PortFormValue = Form.useWatch(["ports", name], form) ?? {};
  const { data: ports = [], isLoading: portsLoading } = useLibrenmsPorts(
    value.deviceId,
  );

  // A new device makes the old port and its label meaningless.
  const handleDevice = (deviceId: number) => {
    const device = devices.find((d) => d.deviceId === deviceId);
    form.setFieldValue(["ports", name], {
      deviceId,
      deviceName: device ? deviceDisplayName(device) : String(deviceId),
      ifName: undefined,
      label: "",
    });
  };

  const handlePort = (ifName: string) => {
    const port = ports.find((p) => p.ifName === ifName);
    form.setFieldValue(
      ["ports", name, "label"],
      port ? defaultPortLabel(port) : ifName,
    );
  };

  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      <Form.Item name={[name, "deviceName"]} hidden>
        <Input />
      </Form.Item>
      <Form.Item
        name={[name, "deviceId"]}
        rules={[{ required: true, message: "Pilih perangkat" }]}
        style={FIELD_STYLE}
      >
        <Select
          showSearch
          placeholder="Perangkat"
          optionFilterProp="label"
          loading={devicesLoading}
          options={deviceOptions(devices, value)}
          onChange={handleDevice}
        />
      </Form.Item>
      <Form.Item
        name={[name, "ifName"]}
        rules={[{ required: true, message: "Pilih port" }]}
        style={FIELD_STYLE}
      >
        <Select
          showSearch
          placeholder="Port"
          optionFilterProp="label"
          disabled={value.deviceId === undefined}
          loading={portsLoading}
          options={ports.map((p) => ({
            value: p.ifName,
            label: portOptionLabel(p),
          }))}
          onChange={handlePort}
        />
      </Form.Item>
      <Form.Item
        name={[name, "label"]}
        rules={[
          { required: true, whitespace: true, message: "Isi label" },
          { max: 255 },
        ]}
        style={FIELD_STYLE}
      >
        <Input placeholder="Judul di atas grafik" />
      </Form.Item>
      <PortRowActions
        index={index}
        count={count}
        canPreview={value.deviceId !== undefined && !!value.ifName}
        onMove={onMove}
        onRemove={onRemove}
        onPreview={() =>
          onPreview({
            deviceId: value.deviceId as number,
            ifName: value.ifName as string,
            label: value.label || (value.ifName as string),
          })
        }
      />
    </div>
  );
}
