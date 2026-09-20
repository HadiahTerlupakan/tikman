import { Alert, Skeleton, Space } from "antd";
import { Link } from "react-router-dom";
import type { MappingEdge, MappingNode, Waypoint } from "@/domain/entities";
import { EdgeList } from "./EdgeList";
import { MapCanvas } from "./MapCanvas";
import type { MapView, Placing } from "./MapToolbar";
import type { PopupState } from "./MapCanvasPopups";
import { NodeList } from "./NodeList";

interface NetworkMapBodyProps {
  view: MapView;
  keyLoading: boolean;
  apiKey?: string;
  mapId?: string;
  nodes: MappingNode[];
  edges: MappingEdge[];
  tracing: {
    draft: Waypoint[];
    fromNodeId?: string;
    redrawingEdgeId?: string;
  };
  placing: Placing | undefined;
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
  onEdgeClick: (edgeId: string) => void;
  popup: PopupState;
}

// The Daftar tables. Reuses `popup.actions` for its own Ubah/Hapus/Gambar
// ulang buttons rather than taking a second set of the same five callbacks,
// since both surfaces call the exact same handlers.
function NetworkMapListView({
  nodes,
  edges,
  popup,
}: Pick<NetworkMapBodyProps, "nodes" | "edges" | "popup">) {
  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <NodeList
        nodes={nodes}
        onEdit={popup.actions.onEditNode}
        onDelete={popup.actions.onDeleteNode}
      />
      <EdgeList
        edges={edges}
        onEdit={popup.actions.onEditEdge}
        onRedraw={popup.actions.onRedrawEdge}
        onDelete={popup.actions.onDeleteEdge}
      />
    </Space>
  );
}

function NoMapsKeyNotice() {
  return (
    <Alert
      type="info"
      showIcon
      message="Kunci API Google Maps belum diatur"
      description={
        <span>
          Peta tidak bisa digambar tanpa kunci. Tambahkan di{" "}
          <Link to="/settings">Pengaturan</Link>.
        </span>
      }
    />
  );
}

/**
 * Either the map or the Daftar tables, depending on the toolbar's view
 * switch — split out of NetworkMapPage purely to keep that file under the
 * project's line limit; this owns no state of its own.
 */
export function NetworkMapBody({
  view,
  keyLoading,
  apiKey,
  mapId,
  nodes,
  edges,
  tracing,
  placing,
  onDrop,
  onNodeClick,
  onEdgeClick,
  popup,
}: NetworkMapBodyProps) {
  if (view !== "map") {
    return <NetworkMapListView nodes={nodes} edges={edges} popup={popup} />;
  }
  if (keyLoading) {
    return <Skeleton active paragraph={{ rows: 8 }} title={false} />;
  }
  if (!apiKey) {
    return <NoMapsKeyNotice />;
  }

  return (
    <MapCanvas
      nodes={nodes}
      edges={edges}
      tracing={tracing}
      placing={placing}
      apiKey={apiKey}
      mapId={mapId}
      onDrop={onDrop}
      onNodeClick={onNodeClick}
      onEdgeClick={onEdgeClick}
      popup={popup}
    />
  );
}
