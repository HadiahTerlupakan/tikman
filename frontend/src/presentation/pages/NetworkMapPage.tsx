import { useState } from "react";
import { Alert, Button, Skeleton, Space, message } from "antd";
import { Link } from "react-router-dom";
import type {
  FiberType,
  MappingNode,
  NodeType,
  Waypoint,
} from "@/domain/entities";
import {
  useCreateEdge,
  useCreateNode,
  useDeleteEdge,
  useDeleteNode,
  useGoogleMapsKey,
  useMappingEdges,
  useMappingNodes,
  useUpdateNode,
} from "@/application/hooks";
import { PageHeader } from "../components/common";
import { CableTypeModal } from "../components/netmap/CableTypeModal";
import { CountCards } from "../components/netmap/CountCards";
import { EdgeList } from "../components/netmap/EdgeList";
import { MapCanvas } from "../components/netmap/MapCanvas";
import { MapToolbar, type MapView } from "../components/netmap/MapToolbar";
import { NodeFormModal } from "../components/netmap/NodeFormModal";
import { NodeList } from "../components/netmap/NodeList";
import { useCableDraw } from "../components/netmap/useCableDraw";

// What the node form is doing right now: adding a freshly tapped point.
// `initial` is set later (editing) once the list view can open it.
interface NodeFormTarget {
  type: NodeType;
  position: Waypoint;
  initial?: MappingNode;
}

// A cable with both ends named and its path traced, waiting only on which of
// the seven fiber types it is before it can be saved.
interface PendingCable {
  source: string;
  target: string;
  waypoints: Waypoint[];
  distance: number;
}

export function NetworkMapPage() {
  const { data: nodes = [] } = useMappingNodes();
  const { data: edges = [] } = useMappingEdges();
  const createNode = useCreateNode();
  const updateNode = useUpdateNode();
  const deleteNode = useDeleteNode();
  const createEdge = useCreateEdge();
  const deleteEdge = useDeleteEdge();
  const { key, mapId, isLoading: keyLoading } = useGoogleMapsKey();
  const cable = useCableDraw();

  const [view, setView] = useState<MapView>("map");
  const [placing, setPlacing] = useState<NodeType | "cable">();
  const [formTarget, setFormTarget] = useState<NodeFormTarget>();
  const [pendingCable, setPendingCable] = useState<PendingCable>();

  const stopPlacing = () => {
    setPlacing(undefined);
    setFormTarget(undefined);
    setPendingCable(undefined);
    cable.cancel();
  };

  // A tap on the map means "put a node here" while placing a node, and "one
  // more corner" while a cable is being traced.
  const tapped = (point: Waypoint) => {
    if (placing === "cable") {
      cable.addPoint(point);
      return;
    }
    if (placing) {
      setFormTarget({ type: placing, position: point });
    }
  };

  // The first node tapped starts the cable; the second ends it and asks which
  // of the seven fiber types it is before anything is saved.
  const nodeTapped = (nodeId: string) => {
    if (placing !== "cable") {
      return;
    }
    if (!cable.from) {
      cable.start(nodeId);
      return;
    }
    const source = cable.from;
    const distance = Math.round(cable.meters);
    const waypoints = cable.finish();
    setPendingCable({ source, target: nodeId, waypoints, distance });
  };

  const saveNode = async (node: MappingNode) => {
    try {
      if (formTarget?.initial) {
        await updateNode.mutateAsync({ nodeId: node.nodeId, node });
      } else {
        await createNode.mutateAsync(node);
      }
      setFormTarget(undefined);
      setPlacing(undefined);
    } catch {
      message.error("Gagal menyimpan node");
    }
  };

  const saveCable = async (fiberType: FiberType) => {
    if (!pendingCable) {
      return;
    }
    try {
      await createEdge.mutateAsync({
        edgeId: `${pendingCable.source}--${pendingCable.target}`,
        source: pendingCable.source,
        target: pendingCable.target,
        fiberType,
        distance: pendingCable.distance,
        waypoints: pendingCable.waypoints,
        notes: "",
      });
      message.success("Kabel tersimpan");
      setPlacing(undefined);
    } catch {
      // The id is `source--target`, so a second cable between the same pair
      // is a 409 the operator needs in plain words, not the raw API error.
      message.error("Sudah ada kabel antara kedua node ini");
    } finally {
      setPendingCable(undefined);
    }
  };

  // Editing opens the same form as placing a new node, seeded from the node's
  // own position instead of a map tap.
  const editNode = (node: MappingNode) => {
    setFormTarget({
      type: node.type,
      position: { lat: node.latitude, lng: node.longitude },
      initial: node,
    });
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <PageHeader title="Peta Jaringan" />
      <MapToolbar
        placing={placing}
        onPlace={setPlacing}
        onDrawCable={() => setPlacing("cable")}
        onCancel={stopPlacing}
        view={view}
        onView={setView}
      />
      {placing === "cable" && (
        <Button onClick={cable.undoPoint} disabled={cable.points.length === 0}>
          Batal titik
        </Button>
      )}
      {view === "map" ? (
        keyLoading ? (
          <Skeleton active paragraph={{ rows: 8 }} title={false} />
        ) : !key ? (
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
        ) : (
          <MapCanvas
            nodes={nodes}
            edges={edges}
            draft={cable.points}
            placing={placing}
            apiKey={key}
            mapId={mapId}
            onDrop={tapped}
            onNodeClick={nodeTapped}
          />
        )
      ) : (
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <NodeList
            nodes={nodes}
            onEdit={editNode}
            onDelete={(nodeId) => deleteNode.mutateAsync(nodeId)}
          />
          <EdgeList
            edges={edges}
            onDelete={(edgeId) => deleteEdge.mutateAsync(edgeId)}
          />
        </Space>
      )}
      <CountCards nodes={nodes} />
      {formTarget && (
        <NodeFormModal
          open
          type={formTarget.type}
          position={formTarget.position}
          initial={formTarget.initial}
          onCancel={() => setFormTarget(undefined)}
          onSubmit={saveNode}
        />
      )}
      {pendingCable && (
        <CableTypeModal
          open
          onCancel={() => setPendingCable(undefined)}
          onSubmit={saveCable}
        />
      )}
    </Space>
  );
}
