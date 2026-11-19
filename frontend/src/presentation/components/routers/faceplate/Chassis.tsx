import type { CSSProperties } from "react";
import { formatBps } from "../routerTraffic";
import {
  CHASSIS_HEIGHT,
  type FaceplateLayout,
  type PlacedJack,
} from "./portLayout";
import { STATE_TEXT, type PortRole, type PortView } from "./portModel";
// Jacks, LEDs and the blink need pseudo-elements and a keyframe, which inline styles cannot express.
import "./faceplate.css";

const PLUG_COLOR: Record<PortRole, string> = {
  upstream: "var(--fp-tape-upstream)",
  dist: "var(--fp-tape-dist-plug)",
  mgmt: "var(--fp-tape-mgmt)",
};

const BLINK_THRESHOLD_BPS = 1e6;
const NUM_OFFSET_RJ = 14;
const NUM_OFFSET_CAGE = 22;
const NUM_LINE = 10;

interface ChassisProps {
  layout: FaceplateLayout;
  ports: Map<string, PortView>;
  offline: boolean;
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

// More inbound traffic blinks faster: about 1 s at 1 Mbps, the 0.12 s floor near 30 Gbps.
function blinkPeriod(inBps: number): string {
  return `${Math.max(0.12, 1.2 - Math.log10(inBps) / 9).toFixed(2)}s`;
}

function portNumber(jack: PlacedJack): string {
  const short = jack.name
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
  const act = live && view !== undefined && view.inBps > BLINK_THRESHOLD_BPS;
  const blink = act
    ? ({ "--fp-blink": blinkPeriod(view.inBps) } as CSSProperties)
    : undefined;
  if (jack.kind !== "rj") {
    return <i className={ledClass("fp-led", link, act)} style={blink} />;
  }
  const lit =
    live && view && view.state !== "down" && view.state !== "disabled";
  return (
    <>
      <i className={ledClass("fp-led fp-led-a", link, false)} />
      <i
        className={ledClass("fp-led fp-led-b", lit ? "up" : "", act)}
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
    return (
      <div
        className={`${cls} fp-free`}
        style={box}
        data-port={jack.name}
        aria-hidden
      >
        <div className="fp-jack" />
        <PortLeds jack={jack} live={false} />
      </div>
    );
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

/** A router's front panel; with onSelect, ports are buttons where something is plugged in. */
export function Chassis({
  layout,
  ports,
  offline,
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
      <div className="fp-silk fp-model">{model}</div>
      <span className="fp-chassis-status" aria-hidden>
        <i className={offline ? undefined : "fp-led-up"} />
        STATUS
      </span>
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
