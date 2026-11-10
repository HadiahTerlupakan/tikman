import { Button, Form, Typography } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import { GRAPH_REPORT_MAX_PORTS, type LibrenmsDevice } from "@/domain/entities";
import { PortRow } from "./PortRow";
import type { PreviewTarget } from "./PortPreviewModal";

interface PortListProps {
  devices: LibrenmsDevice[];
  devicesLoading: boolean;
  onPreview: (target: PreviewTarget) => void;
}

export function PortList({
  devices,
  devicesLoading,
  onPreview,
}: PortListProps) {
  return (
    <Form.List
      name="ports"
      rules={[
        {
          validator: async (_, ports?: unknown[]) => {
            if (!ports || ports.length === 0) {
              throw new Error("Tambahkan minimal satu port");
            }
          },
        },
      ]}
    >
      {(fields, { add, remove, move }, { errors }) => (
        <>
          <Typography.Text strong>
            Port ({fields.length}/{GRAPH_REPORT_MAX_PORTS}) — dikirim sesuai
            urutan
          </Typography.Text>
          <div style={{ marginTop: 8 }}>
            {fields.map((field, index) => (
              <PortRow
                key={field.key}
                name={field.name}
                index={index}
                count={fields.length}
                devices={devices}
                devicesLoading={devicesLoading}
                onMove={move}
                onRemove={remove}
                onPreview={onPreview}
              />
            ))}
          </div>
          <Form.ErrorList errors={errors} />
          <Button
            type="dashed"
            icon={<PlusOutlined />}
            disabled={fields.length >= GRAPH_REPORT_MAX_PORTS}
            onClick={() => add({})}
          >
            Tambah port
          </Button>
        </>
      )}
    </Form.List>
  );
}
