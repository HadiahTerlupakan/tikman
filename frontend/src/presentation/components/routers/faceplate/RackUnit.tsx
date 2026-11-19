import { type KeyboardEvent, useEffect, useMemo, useRef } from "react";
import { useNavigate } from "react-router-dom";
import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { RouterStatusTag } from "../RouterStatusTag";
import { ROLE_LABEL, isBras } from "../routerRole";
import { Chassis } from "./Chassis";
import { HEALTH_TICK_MS, healthView, type HealthView } from "./healthModel";
import {
  CHASSIS_HEIGHT,
  layoutFaceplate,
  physicalPortNames,
  type FaceplateLayout,
} from "./portLayout";
import {
  bgpStale,
  buildPortViews,
  routerReachable,
  type PortView,
} from "./portModel";
import { useFitScale } from "./useFitScale";
import { useNow } from "./useNow";
// The rack's rails, blanks and hover ring need pseudo-elements, which inline styles cannot express.
import "./rack.css";

const TAPE_RIGHT = 34;
const TAPE_BOTTOM = 8;

// Stable fallbacks: a fresh [] per render would rebuild the port views every render.
const NO_INTERFACES: RouterInterface[] = [];
const NO_SESSIONS: RouterBgpSession[] = [];

interface RackUnitProps {
  router: MikrotikRouter;
  u: number;
  /** The widest board in the rack; every unit draws at it so they scale alike. */
  sharedWidth: number;
  isAdmin: boolean;
  onEdit: (r: MikrotikRouter) => void;
  onLayout: (id: string, width: number) => void;
}

interface Status {
  text: string;
  tone: "" | "warn" | "down" | "none";
}

/** Only drawn ports count: a down tunnel or a quiet bridge has no LED to match. */
function issues(
  router: MikrotikRouter,
  used: PortView[],
  health: HealthView,
): string[] {
  const down = used.filter((p) => p.state === "down").length;
  const silent = used.filter((p) => p.state === "silent").length;
  const dead =
    isBras(router) || bgpStale(router)
      ? 0
      : router.bgpTotal - router.bgpEstablished;
  const found: string[] = [];
  if (down) found.push(`${down} port mati`);
  if (silent) found.push(`${silent} port diam`);
  if (dead) found.push(`${dead} sesi BGP mati`);
  if (health.fresh) found.push(...health.failures.map((f) => `${f} fail`));
  return found;
}

/** null while interfaces load, so nothing is claimed before there is data. */
function unitStatus(
  router: MikrotikRouter,
  ports: Map<string, PortView>,
  physical: string[],
  health: HealthView,
  query: { loading: boolean; failed: boolean },
): Status | null {
  if (!routerReachable(router))
    return { text: "Tidak terjangkau", tone: "down" };
  if (query.loading) return null;
  if (query.failed)
    return { text: "Data interface gagal dimuat", tone: "warn" };
  if (physical.length === 0) {
    return { text: "Belum ada data interface", tone: "none" };
  }
  const used = physical.flatMap((name) => {
    const view = ports.get(name);
    return view && view.role !== null ? [view] : [];
  });
  const found = issues(router, used, health);
  return found.length
    ? { text: found.join(", "), tone: "warn" }
    : { text: "Semua normal", tone: "" };
}

function StatusLine(props: { router: MikrotikRouter; status: Status | null }) {
  const { router, status } = props;
  if (!status) return null;
  return (
    <span className={status.tone ? `fp-st fp-st-${status.tone}` : "fp-st"}>
      <i />
      <span className="fp-st-text">{status.text}</span>
      {status.tone === "down" && <RouterStatusTag router={router} />}
    </span>
  );
}

function detailLine(router: MikrotikRouter, model: string): string {
  const parts = [model];
  if (router.bras) {
    parts.push(`${router.bras.sessions.toLocaleString("id-ID")} online`);
  }
  if (router.cpuLoad !== null) parts.push(`CPU ${router.cpuLoad}%`);
  if (isBras(router)) return parts.filter(Boolean).join(" · ");
  if (bgpStale(router)) parts.push("BGP usang");
  else if (router.bgpTotal > 0)
    parts.push(`BGP ${router.bgpEstablished}/${router.bgpTotal}`);
  return parts.filter(Boolean).join(" · ");
}

function UnitBay(props: {
  router: MikrotikRouter;
  u: number;
  layout: FaceplateLayout;
  ports: Map<string, PortView>;
  health: HealthView;
  status: Status | null;
}) {
  const { router, u, layout, ports, health, status } = props;
  const navigate = useNavigate();
  const unitRef = useRef<HTMLDivElement>(null);
  const scale = useFitScale(unitRef, layout.width);
  const offline = !routerReachable(router);
  const open = () => navigate(`/routers/${router.id}`);
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") open();
  };
  const label = status
    ? `Buka ${router.name}, ${status.text}`
    : `Buka ${router.name}`;
  return (
    <div className="fp-bay">
      <span className="fp-u">{u}</span>
      <div
        ref={unitRef}
        role="link"
        tabIndex={0}
        aria-label={label}
        className={offline ? "fp-unit fp-unit-offline" : "fp-unit"}
        style={{ height: CHASSIS_HEIGHT * scale }}
        onClick={open}
        onKeyDown={onKey}
      >
        <div
          className="fp-unit-chassis"
          style={{ transform: `scale(${scale})` }}
        >
          <Chassis
            layout={layout}
            ports={ports}
            offline={offline}
            health={health}
          />
        </div>
        <span
          className="fp-tapes"
          style={{ right: TAPE_RIGHT * scale, bottom: TAPE_BOTTOM * scale }}
        >
          <span className={`fp-roletape fp-roletape-${router.role}`}>
            {ROLE_LABEL[router.role]}
          </span>
          <span className="fp-ptape">{router.name}</span>
        </span>
      </div>
    </div>
  );
}

/** One router in the rack: its scaled front panel, and beside it what needs a look. */
export function RackUnit(props: RackUnitProps) {
  const { router, u, sharedWidth, isAdmin, onEdit, onLayout } = props;
  const {
    data: interfaces = NO_INTERFACES,
    isLoading,
    isError,
  } = useRouterInterfaces(router.id);
  const { data: sessions = NO_SESSIONS } = useRouterBgpSessions(
    router.id,
    !isBras(router),
  );
  const now = useNow(HEALTH_TICK_MS);
  const physical = useMemo(() => physicalPortNames(interfaces), [interfaces]);
  const layout = useMemo(
    () => layoutFaceplate(router.boardName, physical),
    [router.boardName, physical],
  );
  const ports = useMemo(
    () => buildPortViews(router, interfaces, sessions),
    [router, interfaces, sessions],
  );
  useEffect(() => {
    onLayout(router.id, layout.width);
  }, [onLayout, router.id, layout.width]);
  const shared = { ...layout, width: Math.max(sharedWidth, layout.width) };
  const detail = detailLine(router, layout.model);
  const health = healthView(router, now);
  const status = unitStatus(router, ports, physical, health, {
    loading: isLoading,
    failed: isError,
  });

  return (
    <div className="fp-row">
      <UnitBay
        router={router}
        u={u}
        layout={shared}
        ports={ports}
        health={health}
        status={status}
      />
      <div className="fp-info">
        <div className="fp-info-head">
          <span className="fp-info-name">{router.name}</span>
          {isAdmin && (
            <button
              type="button"
              className="fp-info-edit"
              onClick={() => onEdit(router)}
            >
              Ubah
            </button>
          )}
        </div>
        <StatusLine router={router} status={status} />
        <span className="fp-info-muted" title={detail}>
          {detail}
        </span>
      </div>
    </div>
  );
}
