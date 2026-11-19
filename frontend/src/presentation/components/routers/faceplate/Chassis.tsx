import type { CSSProperties } from "react";
import { formatBps } from "../routerTraffic";
import {
  CHASSIS_HEIGHT,
  type FaceplateLayout,
  type PlacedJack,
} from "./portLayout";
import type { HealthView } from "./healthModel";
import { HealthLeds } from "./HealthLeds";
import { STATE_TEXT, type PortRole, type PortView } from "./portModel";
// Jacks, LEDs and the blink need pseudo-elements and a keyframe, which inline styles cannot express.
import "./faceplate.css";
import "./portLeds.css";

const PLUG_COLOR: Record<PortRole, string> = {
  upstream: "var(--fp-tape-upstream)",
  dist: "var(--fp-tape-dist-plug)",
  mgmt: "var(--fp-tape-mgmt)",
};

// Anything above nothing lights the activity LED: a port carrying only ARP and
// discovery still blips now and then on the router.
const ACTIVITY_FLOOR_BPS = 1;
// Below this the port carries only background chatter: an occasional blip.
const TRICKLE_BPS = 10e3;
const NUM_OFFSET_RJ = 14;
const NUM_OFFSET_CAGE = 22;
const NUM_LINE = 10;
const MODEL_RIGHT = 34;
const MODEL_GAP = 16;

interface ChassisProps {
  layout: FaceplateLayout;
  ports: Map<string, PortView>;
  offline: boolean;
  health: HealthView;
  selected?: string;
  onSelect?: (name: string) => void;
}

/** One stripe per role, so a trunk shows everything it carries. */
function plugFill(roles: PortRole[]): string {
  if (roles.length === 1) return PLUG_COLOR[roles[0]];
  const step = 100 / roles.length;
  const stops = roles.map(
    (r, i) => `${PLUG_COLOR[r]} ${i * step}% ${(i + 1) * step}%`,
  );
  return `linear-gradient(90deg, ${stops.join(", ")})`;
}

/** A dead BGP session behind a port that is up still needs a look, so it reads amber. */
function linkLed(view: PortView): string {
  if (view.state === "up") return view.deadSessions > 0 ? "warn" : "up";
  if (view.state === "silent") return "warn";
  return view.state === "down" ? "down" : "";
}

// The busier the port against its own speed, the faster it flickers: a 10G
// port at 1.5 Gbps flickers slower than a 1G port near full. TikMan reads the
// counters once a minute, so this shows the last minute's load, not packets.
const FLICKER_BY_LOAD: [number, number][] = [
  [0.5, 0.22],
  [0.1, 0.4],
  [0.01, 0.8],
  [0, 1.6],
];

function activity(view: PortView): CSSProperties | null {
  const bps = Math.max(view.inBps, view.outBps);
  if (bps < ACTIVITY_FLOOR_BPS) return null;
  const speedBps = (view.iface.speedMbps || 1000) * 1e6;
  const load = bps / speedBps;
  const trickle = bps < TRICKLE_BPS;
  const period = trickle
    ? 3
    : FLICKER_BY_LOAD.find(([min]) => load >= min)?.[1] ?? 1.6;
  return {
    "--fp-blink": `${period}s`,
    "--fp-pattern": trickle ? "fp-blip" : "fp-flicker",
    // Each port starts its pattern at its own point, so the panel never blinks in step.
    "--fp-phase": `-${(hash(view.iface.name) % 1000) / 1000}s`,
  } as CSSProperties;
}

function hash(text: string): number {
  let h = 0;
  for (const ch of text) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return h;
}

function portNumber(jack: PlacedJack): string {
  const short = jack.slot
    .replace("ether", "")
    .replace("qsfp28-", "QSFP28-")
    .replace("sfp-sfpplus", "SFP+")
    .replace(/^sfp28-/, "SFP28-");
  return jack.mgmt ? `${short} MGMT` : short;
}

function ledClass(base: string, state: string, act: boolean): string {
  return [base, state && `fp-led-${state}`, act && "fp-led-act"]
    .filter(Boolean)
    .join(" ");
}

function PortLeds(props: { jack: PlacedJack; view?: PortView; live: boolean }) {
  const { jack, view, live } = props;
  const link = live && view ? linkLed(view) : "";
  const blink = live && view ? activity(view) ?? undefined : undefined;
  const act = blink !== undefined;
  if (jack.kind !== "rj") {
    return <i className={ledClass("fp-led", link, act)} style={blink} />;
  }
  const lit =
    live && view && view.state !== "down" && view.state !== "disabled";
  // As on the operator's routers: the RJ45 link LED glows orange while a cable
  // is up, and the green one flickers with traffic. A port TikMan expects up
  // but finds down stays red, which no hardware LED says.
  const rjLink = link === "down" ? "down" : link ? "link" : "";
  return (
    <>
      <i className={ledClass("fp-led fp-led-a", rjLink, false)} />
      <i
        className={ledClass("fp-led fp-led-b", lit && act ? "up" : "", act)}
        style={blink}
      />
    </>
  );
}

// An unreachable router's state and rate are last-known, so neither is named as if live.
function stateText(view: PortView, offline: boolean): string {
  return offline ? "Tidak terjangkau" : STATE_TEXT[view.state];
}

function hoverTitle(name: string, view: PortView, offline: boolean): string {
  const base = `${name} · ${view.label} · ${stateText(view, offline)}`;
  return !offline && view.state === "up"
    ? `${base} · ↓ ${formatBps(view.inBps)}`
    : base;
}

function PortFace(props: {
  jack: PlacedJack;
  view: PortView;
  offline: boolean;
}) {
  const { jack, view, offline } = props;
  return (
    <>
      <div className="fp-jack" />
      <span className="fp-plug" style={{ background: plugFill(view.roles) }} />
      <PortLeds jack={jack} view={view} live={!offline} />
    </>
  );
}

function FreePort(props: {
  jack: PlacedJack;
  className: string;
  style: CSSProperties;
}) {
  const { jack, className, style } = props;
  return (
    <div className={className} style={style} data-port={jack.name} aria-hidden>
      <div className="fp-jack" />
      <PortLeds jack={jack} live={false} />
    </div>
  );
}

function Port(props: {
  jack: PlacedJack;
  view?: PortView;
  offline: boolean;
  selected: boolean;
  onSelect?: (name: string) => void;
}) {
  const { jack, view, offline, selected, onSelect } = props;
  const box = { left: jack.x, top: jack.y, width: jack.w, height: jack.h };
  const cls = `fp-port fp-${jack.kind} fp-${jack.row}`;
  if (!view || view.role === null) {
    return <FreePort jack={jack} className={`${cls} fp-free`} style={box} />;
  }
  const face = <PortFace jack={jack} view={view} offline={offline} />;
  // Without onSelect the chassis sits inside a rack unit that is itself the
  // link, so a port there is only a picture; the unit's label names its state.
  if (!onSelect) {
    return (
      <div
        className={cls}
        style={box}
        data-port={jack.name}
        title={hoverTitle(jack.name, view, offline)}
        aria-hidden
      >
        {face}
      </div>
    );
  }
  return (
    <button
      type="button"
      className={selected ? `${cls} fp-selected` : cls}
      style={box}
      data-port={jack.name}
      aria-label={`${jack.name}, ${view.label}, ${stateText(view, offline)}`}
      onClick={() => onSelect(jack.name)}
    >
      {face}
    </button>
  );
}

function PortNumber({ jack }: { jack: PlacedJack }) {
  const offset = jack.kind === "rj" ? NUM_OFFSET_RJ : NUM_OFFSET_CAGE;
  const top =
    jack.row === "top" ? jack.y - offset : jack.y + jack.h + offset - NUM_LINE;
  return (
    <div className="fp-num" style={{ left: jack.x, top, width: jack.w }}>
      {portNumber(jack)}
    </div>
  );
}

function RackMountAndConsole() {
  return (
    <>
      <div className="fp-ear fp-ear-l" />
      <div className="fp-ear fp-ear-r" />
      <div className="fp-silk" style={{ left: 40, top: 30 }}>
        CONSOLE
      </div>
      <div
        className="fp-port fp-rj fp-bottom fp-free"
        style={{ left: 36, top: 46, width: 46, height: 40 }}
        aria-hidden
      >
        <div className="fp-jack" />
      </div>
      <div className="fp-silk" style={{ left: 104, top: 30 }}>
        USB
      </div>
      <div className="fp-usb" aria-hidden />
    </>
  );
}

/**
 * The silkscreen model fits the free area right of the last jack and ends in
 * an ellipsis there: its width depends on the font, and an automatic board's
 * model is long enough to run over its last port label.
 */
function modelStyle(layout: FaceplateLayout): CSSProperties {
  const lastJack = Math.max(0, ...layout.jacks.map((j) => j.x + j.w));
  return {
    right: MODEL_RIGHT,
    maxWidth: layout.width - lastJack - MODEL_RIGHT - MODEL_GAP,
    overflow: "hidden",
    textOverflow: "ellipsis",
  };
}

/** A router's front panel; with onSelect, ports are buttons where something is plugged in. */
export function Chassis({
  layout,
  ports,
  offline,
  health,
  selected,
  onSelect,
}: ChassisProps) {
  const model = layout.templated
    ? layout.model
    : `${layout.model} · susunan otomatis`;
  return (
    <div
      className={layout.templated ? "fp-chassis" : "fp-chassis fp-auto"}
      style={{ width: layout.width, height: CHASSIS_HEIGHT }}
    >
      {layout.extras && <RackMountAndConsole />}
      <div className="fp-silk fp-model" style={modelStyle(layout)}>
        {model}
      </div>
      <HealthLeds offline={offline} health={health} />
      {layout.jacks.map((jack) => (
        <Port
          key={jack.name}
          jack={jack}
          view={ports.get(jack.name)}
          offline={offline}
          selected={selected === jack.name}
          onSelect={onSelect}
        />
      ))}
      {layout.jacks.map((jack) => (
        <PortNumber key={jack.name} jack={jack} />
      ))}
    </div>
  );
}
