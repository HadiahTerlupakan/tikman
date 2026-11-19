import { Typography } from "antd";
import type { RouterBrasView } from "@/domain/entities";
import {
  brasSignals,
  isBrasStale,
  sinceText,
  type Signal,
  type SignalLevel,
} from "./brasSignalModel";
import "../faceplate/faceplate.css";
import "./bras.css";

const LEVEL_TEXT: Record<SignalLevel, string> = {
  ok: "normal",
  warn: "perlu dilihat",
  bad: "bermasalah",
  none: "tidak dinilai",
};

export function BrasLed({ level }: { level: SignalLevel }) {
  return (
    <span
      role="img"
      aria-label={LEVEL_TEXT[level]}
      className={`bras-led bras-led-${level}`}
    />
  );
}

function SignalCell({ title, signal }: { title: string; signal: Signal }) {
  return (
    <div className="bras-signal">
      <Typography.Text type="secondary" className="bras-signal-what">
        <BrasLed level={signal.level} />
        {title}
      </Typography.Text>
      <div className="bras-signal-value">{signal.value}</div>
      <Typography.Text type="secondary" className="bras-signal-context">
        {signal.context}
      </Typography.Text>
    </div>
  );
}

/** The four signals at the top of the summary, greyed out with the reading's age when stale. */
export function BrasSignals({
  view,
  now,
}: {
  view: RouterBrasView;
  now: Date;
}) {
  const s = brasSignals(view, now);
  const stale = isBrasStale(view.brasAt, now);
  return (
    <section aria-label="Kondisi BRAS">
      <div
        className={stale ? "bras-signals bras-signals-stale" : "bras-signals"}
      >
        <SignalCell title="Pelanggan online" signal={s.sessions} />
        <SignalCell title="RADIUS" signal={s.radius} />
        <SignalCell title="CPU core terberat" signal={s.cpu} />
        <SignalCell title="Sisa alamat pelanggan" signal={s.pool} />
      </div>
      {stale && view.brasAt && (
        <Typography.Text type="secondary">
          {`Pembacaan terakhir ${sinceText(view.brasAt, now)}`}
        </Typography.Text>
      )}
      {!view.brasAt && (
        <Typography.Text type="secondary">
          Belum ada pembacaan dari BRAS ini
        </Typography.Text>
      )}
    </section>
  );
}
