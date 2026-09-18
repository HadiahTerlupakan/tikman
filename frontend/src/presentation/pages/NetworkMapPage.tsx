import { useState } from "react";
import { Alert, Button, Skeleton, Space, message } from "antd";
import { Link } from "react-router-dom";
import type {
  FiberType,
  MappingEdge,
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
  useUpdateEdge,
  useUpdateNode,
} from "@/application/hooks";
import { PageHeader } from "../components/common";
import { CableTypeModal } from "../components/netmap/CableTypeModal";
import { cablePath, metersAlong } from "../components/netmap/cableMath";
import { CountCards } from "../components/netmap/CountCards";
import { EdgeFormModal } from "../components/netmap/EdgeFormModal";
import { EdgeList } from "../components/netmap/EdgeList";
import { MapCanvas } from "../components/netmap/MapCanvas";
import {
  errorMessage,
  isEdgeExists,
  isNodeExists,
  isNodeInUse,
} from "../components/netmap/mappingErrors";
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
  const updateEdge = useUpdateEdge();
  const deleteEdge = useDeleteEdge();
  const { key, mapId, isLoading: keyLoading } = useGoogleMapsKey();
  const cable = useCableDraw();

  const [view, setView] = useState<MapView>("map");
  const [placing, setPlacing] = useState<NodeType | "cable">();
  const [formTarget, setFormTarget] = useState<NodeFormTarget>();
  const [pendingCable, setPendingCable] = useState<PendingCable>();
  const [editingEdge, setEditingEdge] = useState<MappingEdge>();

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
  // of the seven fiber types it is before anything is saved. A redraw's
  // endpoints are already fixed by the cable on record, so a node tap has
  // nothing to do here — Selesai (finishRedraw) is its only way to finish,
  // which keeps re-tracing from ever reassigning what the cable connects.
  const nodeTapped = (nodeId: string) => {
    if (placing !== "cable" || cable.redrawing) {
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

  // Entry point from EdgeList: re-tracing an existing cable's route without
  // touching its endpoints, fiber type or notes. The map has to be visible
  // for a technician to tap corners on it, so this switches view even though
  // starting a fresh cable does not force that switch on its own.
  const redrawEdge = (edge: MappingEdge) => {
    setView("map");
    setPlacing("cable");
    cable.startRedraw(edge);
  };

  // Unlike a fresh cable, a redraw never asks which fiber type it is or for
  // new notes — both are untouched by definition, so only the retraced
  // geometry needs saving. cable.finish() (which clears `redrawing`) only
  // runs on success: clearing it eagerly left `placing === "cable"` with
  // `redrawing` already undefined after a rejection, which is exactly the
  // condition that opens nodeTapped's guard and lets the next two node taps
  // start a cable nobody asked for. Reading `cable.points` directly, instead
  // of through finish(), is what lets the corners survive a failed save.
  const finishRedraw = async () => {
    const edge = cable.redrawing;
    if (!edge) {
      return;
    }
    const waypoints = cable.points;
    const distance = Math.round(
      metersAlong(cablePath(nodes, edge.source, edge.target, waypoints)),
    );
    try {
      await updateEdge.mutateAsync({
        edgeId: edge.edgeId,
        edge: { ...edge, waypoints, distance },
      });
      message.success("Jalur kabel tersimpan");
      cable.finish();
      setPlacing(undefined);
    } catch (error) {
      message.error(errorMessage(error));
    }
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
    } catch (error) {
      // Mirrors saveCable: a duplicate id is its own plain-language message,
      // not indistinguishable from a network error.
      message.error(
        isNodeExists(error)
          ? "Kode node sudah dipakai, gunakan kode lain"
          : "Gagal menyimpan node",
      );
    }
  };

  // No Popconfirm or catch existed here before: one misclick in a 20-row
  // table removed a node outright, orphaning every cable drawn to it, and a
  // failed delete left the row sitting there with no explanation.
  const removeNode = async (nodeId: string) => {
    try {
      await deleteNode.mutateAsync(nodeId);
    } catch (error) {
      // DeleteNode refuses with 409 NODE_IN_USE while an ONT still points at
      // this node, naming how many — that has to reach the operator as
      // itself, not the generic fallback.
      message.error(
        isNodeInUse(error) ? errorMessage(error) : "Gagal menghapus node",
      );
    }
  };

  const removeEdge = async (edgeId: string) => {
    try {
      await deleteEdge.mutateAsync(edgeId);
    } catch {
      message.error("Gagal menghapus kabel");
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
      setPendingCable(undefined);
    } catch (error) {
      // The id is `source--target`, so a second cable between the same pair
      // is a 409 the operator needs in plain words. Anything else — a
      // capacity rule on an odp_to_odp/odc_to_odc cascade, a network error —
      // must surface as itself, not be misreported as a duplicate. Clearing
      // pendingCable only on success (not in a finally) keeps the modal open
      // on failure: a network blip should cost one retry click, not the
      // whole traced path.
      message.error(
        isEdgeExists(error)
          ? "Sudah ada kabel antara kedua node ini"
          : errorMessage(error),
      );
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

  const editEdge = (edge: MappingEdge) => {
    setEditingEdge(edge);
  };

  // The modal stays open on failure, the same choice saveCable and saveNode
  // make for their own failures: nothing here needs to be abandoned, just
  // adjusted and retried.
  const saveEdge = async (edge: MappingEdge) => {
    try {
      await updateEdge.mutateAsync({ edgeId: edge.edgeId, edge });
      setEditingEdge(undefined);
    } catch (error) {
      message.error(errorMessage(error));
    }
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
        <Space>
          <Button
            onClick={cable.undoPoint}
            disabled={cable.points.length === 0}
          >
            Batal titik
          </Button>
          {cable.redrawing && (
            <Button type="primary" onClick={finishRedraw}>
              Selesai
            </Button>
          )}
        </Space>
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
            fromNodeId={cable.from}
            redrawingEdgeId={cable.redrawing?.edgeId}
            placing={placing}
            apiKey={key}
            mapId={mapId}
            onDrop={tapped}
            onNodeClick={nodeTapped}
          />
        )
      ) : (
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <NodeList nodes={nodes} onEdit={editNode} onDelete={removeNode} />
          <EdgeList
            edges={edges}
            onEdit={editEdge}
            onRedraw={redrawEdge}
            onDelete={removeEdge}
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
      {editingEdge && (
        <EdgeFormModal
          open
          initial={editingEdge}
          onCancel={() => setEditingEdge(undefined)}
          onSubmit={saveEdge}
        />
      )}
    </Space>
  );
}
