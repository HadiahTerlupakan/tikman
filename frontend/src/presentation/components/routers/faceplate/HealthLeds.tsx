import type { HealthView } from "./healthModel";
// The faceplate's stylesheet is full; the health LEDs and strip have their own.
import "./health.css";

function Led(props: { label: string; state: "" | "up" | "down" }) {
  const { label, state } = props;
  return (
    <span>
      <i className={state ? `fp-led-${state}` : undefined} />
      {label}
    </span>
  );
}

/**
 * STATUS, one PWR per PSU the board reports, and FAULT. A reading that is not
 * fresh lights nothing, so a router that stopped answering never looks healthy.
 */
export function HealthLeds(props: { offline: boolean; health: HealthView }) {
  const { offline, health } = props;
  const live = !offline && health.fresh;
  const title =
    live && health.readings.length > 0
      ? health.readings.map((r) => `${r.label} ${r.text}`).join(" · ")
      : undefined;
  return (
    <div className="fp-leds" title={title} aria-hidden>
      <Led label="STATUS" state={offline ? "" : "up"} />
      {health.psus.map((psu) => (
        <Led
          key={psu.n}
          label={`PWR${psu.n}`}
          state={live ? (psu.ok ? "up" : "down") : ""}
        />
      ))}
      <Led
        label="FAULT"
        state={live && health.failures.length > 0 ? "down" : ""}
      />
    </div>
  );
}
