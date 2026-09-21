import {
  Checkbox,
  Input,
  InputNumber,
  Select,
  Table,
  Tag,
  Tooltip,
} from "antd";
import type { ImportedNode, NodeType } from "@/domain/entities";
import { NODE_LABELS } from "./mappingLabels";

type OnChange = (row: number, patch: Partial<ImportedNode>) => void;

interface ImportNodesTableProps {
  nodes: ImportedNode[];
  onChange: OnChange;
}

// "server" is excluded on purpose: a server node mirrors a real OLT and can
// never be created through import (ImportedNode.blocked) - offering it here
// would let someone pick an option the commit always refuses regardless.
const TYPE_OPTIONS = (Object.keys(NODE_LABELS) as NodeType[])
  .filter((type) => type !== "server")
  .map((value) => ({ value, label: NODE_LABELS[value] }));

function reasonCell(row: ImportedNode) {
  return (
    <>
      {row.reason && <div>{row.reason}</div>}
      {row.conflict && <Tag color="orange">{row.conflictReason}</Tag>}
    </>
  );
}

function includeColumn(onChange: OnChange) {
  return {
    title: "Sertakan",
    width: 110,
    render: (_: unknown, row: ImportedNode) =>
      row.blocked ? (
        <Tooltip title={row.blockedReason}>
          <Tag color="red">Tidak bisa diimpor</Tag>
        </Tooltip>
      ) : (
        <Checkbox
          checked={row.include}
          onChange={(e) => onChange(row.row, { include: e.target.checked })}
        />
      ),
  };
}

function nodeIdColumn(onChange: OnChange) {
  return {
    title: "Kode",
    render: (_: unknown, row: ImportedNode) => (
      <Input
        size="small"
        value={row.nodeId}
        disabled={row.blocked}
        onChange={(e) => onChange(row.row, { nodeId: e.target.value })}
      />
    ),
  };
}

function typeColumn(onChange: OnChange) {
  return {
    title: "Jenis",
    render: (_: unknown, row: ImportedNode) => (
      <Select
        size="small"
        style={{ width: 130 }}
        placeholder="Pilih jenis"
        value={row.type || undefined}
        disabled={row.blocked}
        options={TYPE_OPTIONS}
        onChange={(value: NodeType) => onChange(row.row, { type: value })}
      />
    ),
  };
}

// Lintang/Bujur are editable, not read-only display: a swapped lat/lng pair
// is the commonest defect in a hand-made or third-party file - exactly what
// this feature exists to import - and the person confirming has to be able
// to see and fix it here, not only be told it exists.
function coordinateColumn(
  title: string,
  field: "latitude" | "longitude",
  onChange: OnChange,
) {
  return {
    title,
    render: (_: unknown, row: ImportedNode) => (
      <InputNumber
        size="small"
        style={{ width: 100 }}
        value={row[field]}
        onChange={(value) =>
          onChange(row.row, { [field]: value ?? 0 } as Partial<ImportedNode>)
        }
      />
    ),
  };
}

/**
 * The node half of the import preview: every Point placemark found, with
 * what TikMan believes it is and why (mapping_kml_import_infer.go on the
 * backend), editable before anything is written. A blocked row (a server
 * node) shows why instead of a checkbox - no edit here can make it
 * importable, see the OLT menu instead.
 */
export function ImportNodesTable({ nodes, onChange }: ImportNodesTableProps) {
  if (nodes.length === 0) {
    return null;
  }
  return (
    <Table
      rowKey="row"
      size="small"
      dataSource={nodes}
      pagination={false}
      title={() => `Node (${nodes.length})`}
      columns={[
        includeColumn(onChange),
        nodeIdColumn(onChange),
        typeColumn(onChange),
        { title: "Nama", dataIndex: "name" },
        coordinateColumn("Lintang", "latitude", onChange),
        coordinateColumn("Bujur", "longitude", onChange),
        {
          title: "Alasan / Konflik",
          render: (_: unknown, row: ImportedNode) => reasonCell(row),
        },
      ]}
    />
  );
}
