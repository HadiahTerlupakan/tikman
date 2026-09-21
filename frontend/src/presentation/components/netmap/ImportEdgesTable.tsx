import { Checkbox, Input, Select, Table, Tag } from "antd";
import type { FiberType, ImportedEdge } from "@/domain/entities";
import { FIBER_OPTIONS } from "./mappingLabels";

type OnChange = (row: number, patch: Partial<ImportedEdge>) => void;

// See ImportNodesTable.tsx's own PREVIEW_PAGE_SIZE for the measurement
// behind this: the same unpaginated-table cost applies here, on the same
// screen, for the same reason.
const PREVIEW_PAGE_SIZE = 50;

interface ImportEdgesTableProps {
  edges: ImportedEdge[];
  onChange: OnChange;
}

function reasonCell(row: ImportedEdge) {
  return (
    <>
      {row.reason && <div>{row.reason}</div>}
      {row.conflict && <Tag color="orange">{row.conflictReason}</Tag>}
      {row.unresolved && !row.conflict && (
        <Tag color="gold">Belum lengkap, isi sumber/tujuan</Tag>
      )}
    </>
  );
}

function includeColumn(onChange: OnChange) {
  return {
    title: "Sertakan",
    width: 90,
    render: (_: unknown, row: ImportedEdge) => (
      <Checkbox
        checked={row.include}
        onChange={(e) => onChange(row.row, { include: e.target.checked })}
      />
    ),
  };
}

// Kode, Sumber and Tujuan are the same shape: a plain editable text field
// keyed straight off one ImportedEdge string field. Editable rather than a
// picker limited to nodes already on the map - an import legitimately
// brings a cable without its ends yet (migrations/53_network_mapping.sql),
// and the two may still be corrected here before that node exists at all.
function textColumn(
  title: string,
  field: "edgeId" | "source" | "target",
  onChange: OnChange,
) {
  return {
    title,
    render: (_: unknown, row: ImportedEdge) => (
      <Input
        size="small"
        value={row[field]}
        onChange={(e) =>
          onChange(row.row, {
            [field]: e.target.value,
          } as Partial<ImportedEdge>)
        }
      />
    ),
  };
}

function fiberTypeColumn(onChange: OnChange) {
  return {
    title: "Jenis Serat",
    render: (_: unknown, row: ImportedEdge) => (
      <Select
        size="small"
        style={{ width: 160 }}
        allowClear
        placeholder="Belum ditentukan"
        value={row.fiberType || undefined}
        options={FIBER_OPTIONS}
        onChange={(value: FiberType | undefined) =>
          onChange(row.row, { fiberType: value ?? "" })
        }
      />
    ),
  };
}

/** The cable half of the import preview - see ImportNodesTable for the node half. */
export function ImportEdgesTable({ edges, onChange }: ImportEdgesTableProps) {
  if (edges.length === 0) {
    return null;
  }
  return (
    <Table
      rowKey="row"
      size="small"
      dataSource={edges}
      pagination={{ pageSize: PREVIEW_PAGE_SIZE }}
      title={() => `Kabel (${edges.length})`}
      columns={[
        includeColumn(onChange),
        textColumn("Kode", "edgeId", onChange),
        textColumn("Sumber", "source", onChange),
        textColumn("Tujuan", "target", onChange),
        fiberTypeColumn(onChange),
        {
          title: "Alasan / Konflik",
          render: (_: unknown, row: ImportedEdge) => reasonCell(row),
        },
      ]}
    />
  );
}
