import { useState } from "react";
import { Space, message } from "antd";
import type {
  FiberType,
  MappingEdge,
  MappingNode,
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
import { CableDrawControls } from "../components/netmap/CableDrawControls";
import { cablePath, metersAlong } from "../components/netmap/cableMath";
import { CountCards } from "../components/netmap/CountCards";
import {
  errorMessage,
  INVALID_COORDINATES_MESSAGE,
  isEdgeExists,
  isInvalidCoordinates,
  isNodeExists,
  isNodeInUse,
  isNodeMirrorsOlt,
  missingEndpointMessage,
} from "../components/netmap/mappingErrors";
import {
  MapToolbar,
  type MapView,
  type Placing,
} from "../components/netmap/MapToolbar";
import { NetworkMapBody } from "../components/netmap/NetworkMapBody";
import {
  NetworkMapModals,
  type NodeFormTarget,
  type PendingCable,
} from "../components/netmap/NetworkMapModals";
import { useCableDraw } from "../components/netmap/useCableDraw";
import { useMapSelection } from "../components/netmap/useMapSelection";
import { useOltPlacement } from "../components/netmap/useOltPlacement";

export function NetworkMapPage() {
  const { data: nodes = [], refetch: refetchNodes } = useMappingNodes();
  const { data: edges = [] } = useMappingEdges();
  const createNode = useCreateNode();
  const updateNode = useUpdateNode();
  const deleteNode = useDeleteNode();
  const createEdge = useCreateEdge();
  const updateEdge = useUpdateEdge();
  const deleteEdge = useDeleteEdge();
  const { key, mapId, isLoading: keyLoading } = useGoogleMapsKey();
  const cable = useCableDraw();
  const selection = useMapSelection();

  const [view, setView] = useState<MapView>("map");
  const [placing, setPlacing] = useState<Placing>();
  const [formTarget, setFormTarget] = useState<NodeFormTarget>();
  const [pendingCable, setPendingCable] = useState<PendingCable>();
  const [editingEdge, setEditingEdge] = useState<MappingEdge>();
  const oltPlacement = useOltPlacement(setPlacing, refetchNodes);

  const stopPlacing = () => {
    setPlacing(undefined);
    setFormTarget(undefined);
    setPendingCable(undefined);
    oltPlacement.cancel();
    cable.cancel();
  };

  // A tap on the map means "put a node here" while placing a node, "one more
  // corner" while a cable is being traced, and "here is where this OLT sits"
  // while an OLT is armed.
  const tapped = (point: Waypoint) => {
    if (placing === "cable") {
      cable.addPoint(point);
      return;
    }
    if (placing === "olt") {
      oltPlacement.drop(point);
      return;
    }
    if (placing) {
      setFormTarget({ type: placing, position: point });
    }
  };

  // The first node tapped starts the cable; the second ends it and asks which
  // of the seven fiber types it is. A redraw's endpoints are already fixed,
  // so a tap does nothing there — Selesai (finishRedraw) is its only way to
  // finish, keeping re-tracing from ever reassigning what a cable connects.
  // Outside all of that, a tap is a technician asking what this box is.
  const nodeTapped = (nodeId: string) => {
    if (placing === "cable") {
      if (cable.redrawing) {
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
      return;
    }
    const node = nodes.find((n) => n.nodeId === nodeId);
    if (node) {
      selection.selectNode(node);
    }
  };

  // Mirrors nodeTapped's guard: tracing owns every tap on the map.
  const edgeTapped = (edgeId: string) => {
    if (placing === "cable") {
      return;
    }
    const edge = edges.find((e) => e.edgeId === edgeId);
    if (edge) {
      selection.selectEdge(edge);
    }
  };

  // Entry point from EdgeList: re-tracing a cable's route without touching
  // its endpoints, fiber type or notes. Switches to the map (unlike starting
  // a fresh cable) since a technician needs it visible to tap corners on it.
  const redrawEdge = (edge: MappingEdge) => {
    setView("map");
    setPlacing("cable");
    cable.startRedraw(edge);
  };

  // A redraw never asks which fiber type it is or for new notes — both are
  // untouched by definition. cable.finish() clears `redrawing`, so it must
  // run only on success: clearing it eagerly (before the mutation settles)
  // is what let a failed save leave `placing` armed with `redrawing` gone,
  // silently reopening nodeTapped's guard for the next two taps.
  const finishRedraw = async () => {
    const edge = cable.redrawing;
    if (!edge) {
      return;
    }
    const missingEndpoint = missingEndpointMessage(nodes, edge);
    if (missingEndpoint) {
      message.error(missingEndpoint);
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
      // and so is a mirror node's coordinates falling outside the range the
      // OLT record itself enforces (UpdateNode's validateCoordinates) —
      // neither is indistinguishable from a plain network error.
      message.error(
        isNodeExists(error)
          ? "Kode node sudah dipakai, gunakan kode lain"
          : isInvalidCoordinates(error)
            ? INVALID_COORDINATES_MESSAGE
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
      // DeleteNode refuses a mirror node with NODE_MIRRORS_OLT and a node
      // still in use with NODE_IN_USE (naming how many ONTs) — both already
      // carry the right plain-language explanation, unlike an unrelated
      // failure.
      message.error(
        isNodeMirrorsOlt(error) || isNodeInUse(error)
          ? errorMessage(error)
          : "Gagal menghapus node",
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
      // must surface as itself, not be misreported as a duplicate.
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
        onPlaceOlt={oltPlacement.arm}
        onDrawCable={() => setPlacing("cable")}
        onCancel={stopPlacing}
        view={view}
        onView={setView}
      />
      {placing === "cable" && (
        <CableDrawControls
          canUndo={cable.points.length > 0}
          onUndo={cable.undoPoint}
          showFinish={Boolean(cable.redrawing)}
          onFinish={finishRedraw}
        />
      )}
      <NetworkMapBody
        view={view}
        keyLoading={keyLoading}
        apiKey={key}
        mapId={mapId}
        nodes={nodes}
        edges={edges}
        tracing={{
          draft: cable.points,
          fromNodeId: cable.from,
          redrawingEdgeId: cable.redrawing?.edgeId,
        }}
        placing={placing}
        onDrop={tapped}
        onNodeClick={nodeTapped}
        onEdgeClick={edgeTapped}
        popup={{
          selectedNode: selection.node,
          selectedEdge: selection.edge,
          onClose: selection.clear,
          actions: {
            onEditNode: editNode,
            onDeleteNode: removeNode,
            onEditEdge: editEdge,
            onRedrawEdge: redrawEdge,
            onDeleteEdge: removeEdge,
          },
        }}
      />
      <CountCards nodes={nodes} olts={oltPlacement.olts} />
      <NetworkMapModals
        formTarget={formTarget}
        onCancelForm={() => setFormTarget(undefined)}
        onSubmitNode={saveNode}
        pendingCable={pendingCable}
        onCancelCable={() => setPendingCable(undefined)}
        onSubmitCable={saveCable}
        editingEdge={editingEdge}
        onCancelEdge={() => setEditingEdge(undefined)}
        onSubmitEdge={saveEdge}
        oltPlacement={oltPlacement.target}
        unplacedOlts={oltPlacement.unplaced}
        onCancelOltPlacement={oltPlacement.cancel}
        onSubmitOltPlacement={oltPlacement.submit}
      />
    </Space>
  );
}
