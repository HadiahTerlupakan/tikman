import { useCallback, useRef, useState } from "react";
import type { MikrotikRouter } from "@/domain/entities";
import { CHASSIS_HEIGHT } from "./portLayout";
import { RackUnit } from "./RackUnit";
import { rackSlots } from "./rackOrder";
import { useFitScale } from "./useFitScale";
// The cabinet, its rails and blank panels need pseudo-elements, which inline styles cannot express.
import "./rack.css";

interface RouterRackProps {
  routers: MikrotikRouter[];
  isAdmin: boolean;
  onEdit: (r: MikrotikRouter) => void;
}

/** As tall as a unit at the rack's scale, so the screen counts U like the cabinet does. */
export function BlankPanel({
  u,
  sharedWidth,
}: {
  u: number;
  sharedWidth: number;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const scale = useFitScale(ref, sharedWidth);
  return (
    <div className="fp-row">
      <div className="fp-bay">
        <span className="fp-u">{u}</span>
        <div
          ref={ref}
          className="fp-blank"
          style={{ height: CHASSIS_HEIGHT * scale }}
          data-testid="rack-blank"
        />
      </div>
      <div />
    </div>
  );
}

/** Every router as a 1U unit in one cabinet, in rack order with blanks for empty positions. */
export function RouterRack({ routers, isAdmin, onEdit }: RouterRackProps) {
  // Each unit learns its board's width only once its interfaces load, so the
  // rack collects them and draws every chassis at the widest one.
  const [widths, setWidths] = useState<Record<string, number>>({});
  const onLayout = useCallback(
    (id: string, width: number) =>
      setWidths((prev) =>
        prev[id] === width ? prev : { ...prev, [id]: width },
      ),
    [],
  );
  const sharedWidth = Math.max(0, ...routers.map((r) => widths[r.id] ?? 0));

  return (
    <div className="fp-cabinet">
      {rackSlots(routers).map((slot) =>
        slot.kind === "blank" ? (
          <BlankPanel
            key={`blank-${slot.u}`}
            u={slot.u}
            sharedWidth={sharedWidth}
          />
        ) : (
          <RackUnit
            key={slot.item.id}
            router={slot.item}
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
