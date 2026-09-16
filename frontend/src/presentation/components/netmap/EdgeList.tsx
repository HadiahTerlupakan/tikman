import { Button, Table } from "antd";
import type { MappingEdge } from "@/domain/entities";
import { formatMeters } from "./cableMath";
import { FIBER_LABELS } from "./mappingLabels";

interface EdgeListProps {
  edges: MappingEdge[];
  onDelete: (edgeId: string) => void;
}

export function EdgeList({ edges, onDelete }: EdgeListProps) {
  return (
    <Table
      rowKey="edgeId"
      dataSource={edges}
      size="small"
      pagination={{ pageSize: 20 }}
      columns={[
        { title: "Kode", dataIndex: "edgeId" },
        { title: "Dari", dataIndex: "source" },
        { title: "Ke", dataIndex: "target" },
        {
          title: "Jenis",
          dataIndex: "fiberType",
          render: (t: MappingEdge["fiberType"]) => FIBER_LABELS[t],
        },
        {
          title: "Panjang",
          dataIndex: "distance",
          render: (d: number) => formatMeters(d),
        },
        {
          title: "",
          render: (_: unknown, edge: MappingEdge) => (
            <Button danger size="small" onClick={() => onDelete(edge.edgeId)}>
              Hapus
            </Button>
          ),
        },
      ]}
    />
  );
}
