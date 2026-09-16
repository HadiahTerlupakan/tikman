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
import { ApiError } from "@/infrastructure/http";
import { PageHeader } from "../components/common";
import { CableTypeModal } from "../components/netmap/CableTypeModal";
import { metersAlong } from "../components/netmap/cableMath";
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

// `useCableDraw.points` holds only the corners tapped between two nodes —
// never the nodes' own positions — so a cable with no corners at all (the
// ordinary drop from an ODP to a house) traces zero of them. The length that
// gets saved has to walk the same source -> corners -> target path
// MapCanvas.edgePath draws, not just the corners.
function cablePath(
  nodes: MappingNode[],
  source: string,
  target: string,
  waypoints: Waypoint[],
): Waypoint[] {
  const point = (nodeId: string): Waypoint[] => {
    const node = nodes.find((n) => n.nodeId === nodeId);
    return node ? [{ lat: node.latitude, lng: node.longitude }] : [];
  };
  return [...point(source), ...waypoints, ...point(target)];
}

// A capacity rule on an odp_to_odp/odc_to_odc cascade must reach the operator
// as itself; only the id collision this cable's own `source--target` naming
// produces should read as "already exists".
function isConflict(error: unknown): boolean {
  return error instanceof ApiError && error.statusCode === 409;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Gagal menyimpan kabel";
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
    const waypoints = cable.finish();
    const distance = Math.round(
      metersAlong(cablePath(nodes, source, nodeId, waypoints)),
    );
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
    } catch (error) {
      // The id is `source--target`, so a second cable between the same pair
      // is a 409 the operator needs in plain words. Anything else — a
      // capacity rule on an odp_to_odp/odc_to_odc cascade, a network error —
      // must surface as itself, not be misreported as a duplicate.
      message.error(
        isConflict(error)
          ? "Sudah ada kabel antara kedua node ini"
          : errorMessage(error),
      );
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
