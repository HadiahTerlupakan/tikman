import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { ManagedSwitch, SwitchInterface } from "@/domain/entities";
import { useSwitchInterfaces } from "@/application/hooks/useSwitches";
import { Chassis } from "../routers/faceplate/Chassis";
import {
  CHASSIS_HEIGHT,
  layoutFaceplate,
  type FaceplateLayout,
} from "../routers/faceplate/portLayout";
import type { PortView } from "../routers/faceplate/portModel";
import { rackSlots } from "../routers/faceplate/rackOrder";
import { BlankPanel } from "../routers/faceplate/RouterRack";
import { useFitScale } from "../routers/faceplate/useFitScale";
import {
  NO_HEALTH,
  SWITCH_PORT_KINDS,
  buildSwitchPortViews,
  physicalSwitchPorts,
  switchReachable,
  switchUnitStatus,
  type SwitchUnitStatus,
} from "./switchPorts";
// The router rack's cabinet, rails and blanks, which need pseudo-elements.
import "../routers/faceplate/rack.css";

const TAPE_RIGHT = 34;
const TAPE_BOTTOM = 8;

const NO_PORTS: SwitchInterface[] = [];

interface SwitchRackProps {
  switches: ManagedSwitch[];
  isAdmin: boolean;
  onEdit: (sw: ManagedSwitch) => void;
}

function StatusLine({ status }: { status: SwitchUnitStatus | null }) {
  if (!status) return null;
  return (
    <span className={status.tone ? `fp-st fp-st-${status.tone}` : "fp-st"}>
      <i />
      <span className="fp-st-text">{status.text}</span>
    </span>
  );
}

function UnitBay(props: {
  sw: ManagedSwitch;
  u: number;
  layout: FaceplateLayout;
  ports: Map<string, PortView>;
}) {
  const { sw, u, layout, ports } = props;
  const unitRef = useRef<HTMLDivElement>(null);
  const scale = useFitScale(unitRef, layout.width);
  const offline = !switchReachable(sw);
  return (
    <div className="fp-bay">
      <span className="fp-u">{u}</span>
      <div
        ref={unitRef}
        className={offline ? "fp-unit fp-unit-offline" : "fp-unit"}
        style={{ height: CHASSIS_HEIGHT * scale }}
      >
        <div
          className="fp-unit-chassis"
          style={{ transform: `scale(${scale})` }}
        >
          <Chassis
            layout={layout}
            ports={ports}
            offline={offline}
            health={NO_HEALTH}
          />
        </div>
        <span
          className="fp-tapes"
          style={{ right: TAPE_RIGHT * scale, bottom: TAPE_BOTTOM * scale }}
        >
          <span className="fp-ptape">{sw.name}</span>
        </span>
      </div>
    </div>
  );
}

function SwitchRackUnit(props: {
  sw: ManagedSwitch;
  u: number;
  sharedWidth: number;
  isAdmin: boolean;
  onEdit: (sw: ManagedSwitch) => void;
  onLayout: (id: string, width: number) => void;
}) {
  const { sw, u, sharedWidth, isAdmin, onEdit, onLayout } = props;
  const { data = NO_PORTS, isLoading, isError } = useSwitchInterfaces(sw.id);
  const physical = useMemo(() => physicalSwitchPorts(data), [data]);
  const layout = useMemo(
    () => layoutFaceplate(sw.model, physical, SWITCH_PORT_KINDS[sw.vendor]),
    [sw.model, sw.vendor, physical],
  );
  const ports = useMemo(() => buildSwitchPortViews(data), [data]);
  useEffect(() => {
    onLayout(sw.id, layout.width);
  }, [onLayout, sw.id, layout.width]);
  const status = switchUnitStatus(switchReachable(sw), ports, physical, {
    loading: isLoading,
    failed: isError,
  });
  const detail = [layout.model, sw.version].filter(Boolean).join(" · ");
  const shared = { ...layout, width: Math.max(sharedWidth, layout.width) };

  return (
    <div className="fp-row">
      <UnitBay sw={sw} u={u} layout={shared} ports={ports} />
      <div className="fp-info">
        <div className="fp-info-head">
          <Link className="fp-info-name" to={`/switches/${sw.id}`}>
            {sw.name}
          </Link>
          {isAdmin && (
            <button
              type="button"
              className="fp-info-edit"
              onClick={() => onEdit(sw)}
            >
              Ubah
            </button>
          )}
        </div>
        <StatusLine status={status} />
        <span className="fp-info-muted" title={detail}>
          {detail}
        </span>
      </div>
    </div>
  );
}

/** Every switch as a 1U unit in one cabinet, drawn like the router rack. */
export function SwitchRack({ switches, isAdmin, onEdit }: SwitchRackProps) {
  // Each unit learns its width once its ports load; all draw at the widest.
  const [widths, setWidths] = useState<Record<string, number>>({});
  const onLayout = useCallback(
    (id: string, width: number) =>
      setWidths((prev) =>
        prev[id] === width ? prev : { ...prev, [id]: width },
      ),
    [],
  );
  const sharedWidth = Math.max(0, ...switches.map((s) => widths[s.id] ?? 0));

  return (
    <div className="fp-cabinet">
      {rackSlots(switches).map((slot) =>
        slot.kind === "blank" ? (
          <BlankPanel
            key={`blank-${slot.u}`}
            u={slot.u}
            sharedWidth={sharedWidth}
          />
        ) : (
          <SwitchRackUnit
            key={slot.item.id}
            sw={slot.item}
            u={slot.u}
            sharedWidth={sharedWidth}
            isAdmin={isAdmin}
            onEdit={onEdit}
            onLayout={onLayout}
          />
        ),
      )}
    </div>
  );
}
