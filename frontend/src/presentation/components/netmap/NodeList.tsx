import { Button, Popconfirm, Space, Table } from "antd";
import type { MappingNode } from "@/domain/entities";
import { NODE_LABELS } from "./mappingLabels";

interface NodeListProps {
  nodes: MappingNode[];
  onEdit: (node: MappingNode) => void;
  onDelete: (nodeId: string) => void;
}

export function NodeList({ nodes, onEdit, onDelete }: NodeListProps) {
  return (
    <Table
      rowKey="nodeId"
      dataSource={nodes}
      size="small"
      pagination={{ pageSize: 20 }}
      columns={[
        { title: "Kode", dataIndex: "nodeId" },
        { title: "Nama", dataIndex: "name" },
        {
          title: "Jenis",
          dataIndex: "type",
          render: (t: MappingNode["type"]) => NODE_LABELS[t],
        },
        { title: "Slot", dataIndex: "capacity" },
        {
          title: "",
          render: (_: unknown, node: MappingNode) => (
            <Space size="small">
              <Button size="small" onClick={() => onEdit(node)}>
                Ubah
              </Button>
              <Popconfirm
                title="Hapus node ini?"
                onConfirm={() => onDelete(node.nodeId)}
                okText="Ya"
                cancelText="Tidak"
              >
                <Button danger size="small">
                  Hapus
                </Button>
              </Popconfirm>
            </Space>
          ),
        },
      ]}
    />
  );
}
