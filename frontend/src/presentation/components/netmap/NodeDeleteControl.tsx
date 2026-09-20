import { Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import type { MappingNode } from "@/domain/entities";

interface NodeDeleteControlProps {
  node: MappingNode;
  onDelete: (nodeId: string) => void;
  /** Run after the OLT-menu link is clicked — NodePopup uses this to close
   * itself, the same as it does after Hapus is confirmed; NodeList's table
   * row has nothing to close, so it leaves this out. */
  onOltLinkClick?: () => void;
}

/**
 * The one control that deletes a node, or explains why it can't — shared by
 * every surface that offers to delete one (today: NodePopup and NodeList),
 * so the rule ("an OLT-backed node cannot be deleted from the map") is
 * enforced once in code rather than copied by hand into each surface, where
 * a copy is exactly what a future surface could forget.
 *
 * A node with oltId set is a mirror of a real OLT record, not a pin —
 * deleting it here would read as removing a marker but would actually
 * strand the OLT and every ONT under it, which DeleteNode already refuses
 * server-side with NODE_MIRRORS_OLT. Pointing at the OLT menu up front
 * means the operator never sees that refusal at all.
 */
export function NodeDeleteControl({
  node,
  onDelete,
  onOltLinkClick,
}: NodeDeleteControlProps) {
  if (node.oltId) {
    return (
      <Link to="/olts" onClick={onOltLinkClick}>
        <Button size="small">Buka menu OLT</Button>
      </Link>
    );
  }

  return (
    <Popconfirm
      title="Hapus node ini?"
      okText="Ya"
      cancelText="Tidak"
      onConfirm={() => onDelete(node.nodeId)}
    >
      <Button size="small" danger>
        Hapus
      </Button>
    </Popconfirm>
  );
}
