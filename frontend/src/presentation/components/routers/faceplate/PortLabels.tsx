import { type CSSProperties, useLayoutEffect, useRef, useState } from "react";
import { formatBps } from "../routerTraffic";
import { Chassis } from "./Chassis";
import type { HealthView } from "./healthModel";
import {
  CHASSIS_HEIGHT,
  type FaceplateLayout,
  type PlacedJack,
} from "./portLayout";
import type { PortView, VlanView } from "./portModel";
import { stageScale, useFitScale } from "./useFitScale";
// Labels, VLAN stripes and the cable layer share the faceplate's pseudo-element styles.
import "./faceplate.css";

const MARGIN = 26;
const CHASSIS_LEFT = 26;
const LANE_GAP = 46;
const TAG_GAP = 10;
const MIN_LEFT = 8;
const BEND = 14;
const BEND_STAGGER = 10;
const CABLE_REACH = 24;
const SAME_X = 2;
const VLANS_SHOWN = 3;

const DEAD_DOT: CSSProperties = {
  width: 7,
  height: 7,
  borderRadius: "50%",
  background: "var(--fp-led-down)",
};
const VLAN_NAME: CSSProperties = {
  minWidth: 0,
  overflow: "hidden",
  textOverflow: "ellipsis",
};
const CABLES: CSSProperties = {
  position: "absolute",
  inset: 0,
  width: "100%",
  height: "100%",
  overflow: "visible",
  pointerEvents: "none",
};

interface Size {
  w: number;
  h: number;
}

interface LabelPos {
  left: number;
  top: number;
}

interface Labelled {
  jack: PlacedJack;
  view: PortView;
}

interface LabelPlacement {
  chassisTop: number;
  width: number;
  height: number;
  tags: Record<string, LabelPos>;
  cables: { port: string; d: string }[];
}

interface Lane {
  top: boolean;
  jacks: PlacedJack[];
  chassisTop: number;
}

function laneHeight(jacks: PlacedJack[], sizes: Record<string, Size>): number {
  return Math.max(0, ...jacks.map((j) => sizes[j.name]?.h ?? 0));
}

// Adjacent cables bend at alternating heights so their horizontal runs do not overlap.
function cablePath(lane: Lane, i: number, cx: number, tag: Size & LabelPos) {
  const chassisEdge = lane.top
    ? lane.chassisTop
    : lane.chassisTop + CHASSIS_HEIGHT;
  const drop = BEND + (i % 2) * BEND_STAGGER;
  const bend = lane.top ? chassisEdge - drop : chassisEdge + drop;
  const end = lane.top ? tag.top + tag.h : tag.top;
  const tx = tag.left + Math.min(tag.w / 2, CABLE_REACH);
  const straight =
    Math.abs(tx - cx) < SAME_X ||
    Math.abs(cx - (tag.left + tag.w / 2)) < SAME_X;
  return `M${cx},${chassisEdge} V${bend} H${straight ? cx : tx} V${end}`;
}

/** Left to right in port order, each label centred on its port unless the one before pushes it. */
function placeLane(
  lane: Lane,
  sizes: Record<string, Size>,
  out: LabelPlacement,
): number {
  let edge = 0;
  let rightmost = 0;
  const sorted = [...lane.jacks].sort((a, b) => a.x - b.x);
  sorted.forEach((jack, i) => {
    const size = sizes[jack.name] ?? { w: 0, h: 0 };
    const cx = CHASSIS_LEFT + jack.x + jack.w / 2;
    const left = Math.max(cx - size.w / 2, edge, MIN_LEFT);
    edge = left + size.w + TAG_GAP;
    rightmost = Math.max(rightmost, left + size.w);
    const top = lane.top
      ? lane.chassisTop - LANE_GAP - size.h
      : lane.chassisTop + CHASSIS_HEIGHT + LANE_GAP;
    out.tags[jack.name] = { left, top };
    out.cables.push({
      port: jack.name,
      d: cablePath(lane, i, cx, { ...size, left, top }),
    });
  });
  return rightmost;
}

/** Top-row ports hang their label above the chassis, every other port below it. */
function placeLabels(
  jacks: PlacedJack[],
  sizes: Record<string, Size>,
  chassisWidth: number,
): LabelPlacement {
  const above = jacks.filter((j) => j.row === "top");
  const below = jacks.filter((j) => j.row !== "top");
  const topH = laneHeight(above, sizes);
  const bottomH = laneHeight(below, sizes);
  const chassisTop = MARGIN + (topH ? topH + LANE_GAP : 0);
  const out: LabelPlacement = {
    chassisTop,
    width: 0,
    height:
      chassisTop + CHASSIS_HEIGHT + (bottomH ? LANE_GAP + bottomH : 0) + MARGIN,
    tags: {},
    cables: [],
  };
  const rightmost = Math.max(
    CHASSIS_LEFT + chassisWidth,
    placeLane({ top: true, jacks: above, chassisTop }, sizes, out),
    placeLane({ top: false, jacks: below, chassisTop }, sizes, out),
  );
  out.width = rightmost + MARGIN;
  return out;
}

function tagSub(view: PortView, offline: boolean): string | null {
  if (offline) return null;
  if (view.state === "down") return "mati";
  if (view.state === "disabled") return "nonaktif";
  if (view.state === "silent") return "tidak ada rx";
  const rate = `↓ ${formatBps(view.inBps)}`;
  return view.vlans.length > 0 ? `${view.vlans.length} VLAN · ${rate}` : rate;
}

function tagClass(view: PortView, offline: boolean, selected: boolean) {
  const live = !offline && (view.state === "down" || view.state === "silent");
  return [
    `fp-tag fp-tag-${view.role}`,
    live && `fp-tag-${view.state}`,
    selected && "fp-tag-selected",
  ]
    .filter(Boolean)
    .join(" ");
}

function VlanLines({
  vlans,
  offline,
}: {
  vlans: VlanView[];
  offline: boolean;
}) {
  return (
    <>
      {vlans.slice(0, VLANS_SHOWN).map((v) => (
        <span
          key={v.iface.id}
          className={v.role ? `fp-vl fp-role-${v.role}` : "fp-vl"}
        >
          <span className="fp-vl-id">{v.iface.vlanId}</span>
          <span style={VLAN_NAME}>{v.label}</span>
          {v.sessions.some((s) => s.state !== "established") && (
            <span
              className="fp-vl-dot"
              style={DEAD_DOT}
              title="Ada sesi BGP mati"
            />
          )}
          <span className="fp-vl-rate">
            {offline ? "" : formatBps(v.inBps)}
          </span>
        </span>
      ))}
      {vlans.length > VLANS_SHOWN && (
        <span className="fp-vl fp-vl-more">{`+${vlans.length - VLANS_SHOWN} VLAN lain`}</span>
      )}
    </>
  );
}

function PortLabel(props: {
  item: Labelled;
  offline: boolean;
  selected: boolean;
  pos?: LabelPos;
  onSelect: (name: string) => void;
}) {
  const { jack, view } = props.item;
  const sub = tagSub(view, props.offline);
  const dead = view.deadSessions ? `, ${view.deadSessions} sesi BGP mati` : "";
  return (
    <button
      type="button"
      data-tag={jack.name}
      className={tagClass(view, props.offline, props.selected)}
      style={props.pos ?? { left: 0, top: 0 }}
      aria-label={`${view.label}, ${jack.name}${dead}`}
      onClick={() => props.onSelect(jack.name)}
    >
      <span className="fp-tag-main">
        {view.label}
        {sub && <small>{sub}</small>}
      </span>
      <VlanLines vlans={view.vlans} offline={props.offline} />
    </button>
  );
}

interface PortLabelsProps {
  layout: FaceplateLayout;
  ports: Map<string, PortView>;
  offline: boolean;
  health: HealthView;
  selected?: string;
  onSelect: (name: string) => void;
}

function labelled(layout: FaceplateLayout, ports: Map<string, PortView>) {
  return layout.jacks.flatMap((jack) => {
    const view = ports.get(jack.name);
    return view && view.role !== null ? [{ jack, view }] : [];
  });
}

/** Labels are placed from their rendered size, so they are measured before the first paint. */
function useLabelPlacement(
  layout: FaceplateLayout,
  ports: Map<string, PortView>,
  offline: boolean,
) {
  const rack = useRef<HTMLDivElement>(null);
  const [placement, setPlacement] = useState<LabelPlacement | null>(null);
  useLayoutEffect(() => {
    const sizes: Record<string, Size> = {};
    rack.current?.querySelectorAll<HTMLElement>("[data-tag]").forEach((el) => {
      sizes[el.dataset.tag ?? ""] = { w: el.offsetWidth, h: el.offsetHeight };
    });
    const jacks = labelled(layout, ports).map((i) => i.jack);
    setPlacement(placeLabels(jacks, sizes, layout.width));
  }, [layout, ports, offline]);
  return { rack, placement };
}

function Cables({ placement }: { placement: LabelPlacement | null }) {
  return (
    <svg style={CABLES} aria-hidden>
      {placement?.cables.map((c) => (
        <path
          key={c.port}
          d={c.d}
          fill="none"
          stroke="var(--fp-cable)"
          strokeWidth={2}
        />
      ))}
    </svg>
  );
}

function LabelLayer(
  props: PortLabelsProps & { placement: LabelPlacement | null },
) {
  const { layout, ports, offline, health, selected, onSelect, placement } =
    props;
  const chassisTop = placement?.chassisTop ?? MARGIN;
  return (
    <>
      <Cables placement={placement} />
      <div
        style={{ position: "absolute", left: CHASSIS_LEFT, top: chassisTop }}
      >
        <Chassis
          layout={layout}
          ports={ports}
          offline={offline}
          health={health}
          selected={selected}
          onSelect={onSelect}
        />
      </div>
      {labelled(layout, ports).map((item) => (
        <PortLabel
          key={item.jack.name}
          item={item}
          offline={offline}
          selected={selected === item.jack.name}
          pos={placement?.tags[item.jack.name]}
          onSelect={onSelect}
        />
      ))}
    </>
  );
}

/** The chassis with a hanging label per used port, cabled to it, scaled down as one to fit its stage. */
export function PortLabels(props: PortLabelsProps) {
  const { rack, placement } = useLabelPlacement(
    props.layout,
    props.ports,
    props.offline,
  );
  const stage = useRef<HTMLDivElement>(null);
  const width = placement?.width ?? 0;
  const height = placement?.height ?? 0;
  const scale = stageScale(useFitScale(stage, width));
  // Labels are placed from offsetWidth, which a transform leaves alone, so
  // scaling the whole layer keeps the placement and shrinks only its footprint.
  const layer: CSSProperties = {
    position: "absolute",
    width: placement?.width,
    height: placement?.height,
    transform: `scale(${scale})`,
    transformOrigin: "0 0",
  };
  const footprint = { width: width * scale, height: height * scale };
  return (
    <div ref={stage}>
      <div style={{ position: "relative", margin: "0 auto", ...footprint }}>
        <div ref={rack} style={layer}>
          <LabelLayer {...props} placement={placement} />
        </div>
      </div>
    </div>
  );
}
