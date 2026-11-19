import {
  type KeyboardEvent,
  type RefObject,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
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
import { Chassis } from "./Chassis";
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

/** The scale that fits a chassis of `width` into the element, kept in step as it resizes. */
function useFitScale(ref: RefObject<HTMLElement | null>, width: number) {
  const [scale, setScale] = useState(1);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const fit = () => setScale(el.clientWidth / width);
    fit();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(fit);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref, width]);
  return scale;
}

function issues(router: MikrotikRouter, ports: Map<string, PortView>) {
  const used = [...ports.values()].filter((p) => p.role !== null);
  const down = used.filter((p) => p.state === "down").length;
  const silent = used.filter((p) => p.state === "silent").length;
  const dead = bgpStale(router) ? 0 : router.bgpTotal - router.bgpEstablished;
  const found: string[] = [];
  if (down) found.push(`${down} port mati`);
  if (silent) found.push(`${silent} port diam`);
  if (dead) found.push(`${dead} sesi BGP mati`);
  return found;
}

function StatusLine(props: {
  router: MikrotikRouter;
  ports: Map<string, PortView>;
}) {
  const { router, ports } = props;
  if (!routerReachable(router)) {
    return (
      <span className="fp-st fp-st-down">
        <i />
        <span>Tidak terjangkau</span>
        <RouterStatusTag router={router} />
      </span>
    );
  }
  const found = issues(router, ports);
  return (
    <span className={found.length ? "fp-st fp-st-warn" : "fp-st"}>
      <i />
      <span>{found.length ? found.join(", ") : "Semua normal"}</span>
    </span>
  );
}

function detailLine(router: MikrotikRouter, model: string): string {
  const parts = [model];
  if (router.cpuLoad !== null) parts.push(`CPU ${router.cpuLoad}%`);
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
}) {
  const { router, u, layout, ports } = props;
  const navigate = useNavigate();
  const unitRef = useRef<HTMLDivElement>(null);
  const scale = useFitScale(unitRef, layout.width);
  const offline = !routerReachable(router);
  const open = () => navigate(`/routers/${router.id}`);
  // Ports inside are buttons; only Enter on the unit itself should open it.
  const onKey = (e: KeyboardEvent) => {
    if (e.target === e.currentTarget && e.key === "Enter") open();
  };
  return (
    <div className="fp-bay">
      <span className="fp-u">{u}</span>
      <div
        ref={unitRef}
        role="link"
        tabIndex={0}
        aria-label={`Buka ${router.name}`}
        className={offline ? "fp-unit fp-unit-offline" : "fp-unit"}
        style={{ height: CHASSIS_HEIGHT * scale }}
        onClick={open}
        onKeyDown={onKey}
      >
        <div
          className="fp-unit-chassis"
          style={{ transform: `scale(${scale})` }}
        >
          <Chassis layout={layout} ports={ports} offline={offline} />
        </div>
        <span
          className="fp-ptape"
          style={{ right: TAPE_RIGHT * scale, bottom: TAPE_BOTTOM * scale }}
        >
          {router.name}
        </span>
      </div>
    </div>
  );
}

/** One router in the rack: its scaled front panel, and beside it what needs a look. */
export function RackUnit(props: RackUnitProps) {
  const { router, u, sharedWidth, isAdmin, onEdit, onLayout } = props;
  const { data: interfaces = NO_INTERFACES } = useRouterInterfaces(router.id);
  const { data: sessions = NO_SESSIONS } = useRouterBgpSessions(router.id);
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

  return (
    <div className="fp-row">
      <UnitBay router={router} u={u} layout={shared} ports={ports} />
      <div className="fp-info">
        <span className="fp-info-name">{router.name}</span>
        <StatusLine router={router} ports={ports} />
        <span className="fp-info-muted">
          {detailLine(router, layout.model)}
        </span>
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
    </div>
  );
}
