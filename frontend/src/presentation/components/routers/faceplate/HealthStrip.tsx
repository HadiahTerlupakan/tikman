import { Typography } from "antd";
import dayjs from "dayjs";
import type { HealthView } from "./healthModel";
import "./health.css";

/** Every item of the router's last health reading; one that is not fresh is dimmed and dated. */
export function HealthStrip(props: {
  health: HealthView;
  healthAt: string | null;
}) {
  const { health, healthAt } = props;
  const stale = !health.fresh && health.readings.length > 0;
  return (
    <section
      aria-label="Kesehatan"
      className={stale ? "fp-health fp-health-stale" : "fp-health"}
    >
      <Typography.Text strong>Kesehatan</Typography.Text>
      {health.readings.length === 0 && (
        <Typography.Text type="secondary">
          Belum ada data kesehatan
        </Typography.Text>
      )}
      {health.readings.map((r) => (
        <span key={r.name} className="fp-health-item">
          {r.label}{" "}
          <b className={r.failed ? "fp-health-fail" : undefined}>{r.text}</b>
        </span>
      ))}
      {stale && healthAt && (
        <Typography.Text type="secondary">
          {`Pembacaan terakhir ${dayjs(healthAt).format("DD MMM HH:mm")}`}
        </Typography.Text>
      )}
    </section>
  );
}
