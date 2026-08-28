import { Card, Spin, Tag } from "antd";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import { Ont } from "@/domain/entities/Ont";
import { useOntTrafficTimeSeries } from "@/application/hooks/useOntMetrics";
import { formatBytes } from "./trafficFormat";
import { percentile95 } from "./trafficStats";

interface OntTrafficCardProps {
  ont: Ont;
  period: string;
  range?: { start: string; end: string; bucket?: "hour" | "day" | "month" };
}

const STATUS_COLORS: Record<string, string> = {
  online: "green",
  offline: "red",
  los: "volcano",
  dying_gasp: "purple",
  unknown: "default",
};

function StatusTag({ status }: { status: string }) {
  return <Tag color={STATUS_COLORS[status] || "default"}>{status}</Tag>;
}

function formatSpeed(mbps: number): string {
  if (mbps >= 1000) {
    return `${(mbps / 1000).toFixed(2)} Gbps`;
  }
  if (mbps >= 1) {
    return `${mbps.toFixed(2)} Mbps`;
  }
  return `${(mbps * 1000).toFixed(2)} Kbps`;
}

function getPeriodDomain(period: string): [number, number] | undefined {
  const value = Number(period.slice(0, -1));
  const unit = period.slice(-1);
  if (!Number.isFinite(value) || value <= 0) {
    return undefined;
  }

  const now = Date.now();
  if (unit === "h") {
    return [now - value * 60 * 60 * 1000, now];
  }
  if (unit === "d") {
    return [now - value * 24 * 60 * 60 * 1000, now];
  }
  return undefined;
}

const AXIS_TICK_COUNT = 5;

// Recharts derives ticks from the data points, not from `domain`, so a range with
// data in only part of it would label just that part and hide how much of the
// selected window is empty. Spacing ticks across the domain keeps the axis honest.
function getAxisTicks(
  domain: [number, number] | undefined,
): number[] | undefined {
  if (!domain) {
    return undefined;
  }
  const [start, end] = domain;
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return undefined;
  }
  const step = (end - start) / (AXIS_TICK_COUNT - 1);
  return Array.from({ length: AXIS_TICK_COUNT }, (_, i) =>
    Math.round(start + step * i),
  );
}

export function OntTrafficCard({ ont, period, range }: OntTrafficCardProps) {
  const { data: series, isLoading } = useOntTrafficTimeSeries(
    ont.id,
    period,
    range,
  );
  const timeSeries = series?.points;
  const usage = series?.usage;
  const isCustomRange = !!range;

  const chartData =
    timeSeries?.map((point) => ({
      time: new Date(point.time).getTime(),
      download: point.txMbps || 0,
      upload: point.rxMbps || 0,
    })) || [];

  // Extract values from time series
  const downloadValues = timeSeries?.map((p) => p.txMbps ?? 0) || [];
  const uploadValues = timeSeries?.map((p) => p.rxMbps ?? 0) || [];
  const downloadPeaks =
    timeSeries?.map((p) => p.txMaxMbps ?? p.txMbps ?? 0) || [];
  const uploadPeaks =
    timeSeries?.map((p) => p.rxMaxMbps ?? p.rxMbps ?? 0) || [];

  // Filter out empty buckets (zero values) - they dilute the average
  const activeDownloadValues = downloadValues.filter((v) => v > 0);
  const activeUploadValues = uploadValues.filter((v) => v > 0);

  // For maximum, use peak values but fall back to regular values if peaks are all zero
  const activeDownloadPeaks = downloadPeaks.filter((p) => p > 0);
  const activeUploadPeaks = uploadPeaks.filter((p) => p > 0);
  const maxDownloadValues =
    activeDownloadPeaks.length > 0 ? activeDownloadPeaks : activeDownloadValues;
  const maxUploadValues =
    activeUploadPeaks.length > 0 ? activeUploadPeaks : activeUploadValues;

  const stats = {
    percentile95Download: percentile95(activeDownloadValues),
    percentile95Upload: percentile95(activeUploadValues),
    download: {
      current:
        downloadValues.length > 0
          ? downloadValues[downloadValues.length - 1]
          : 0,
      average:
        activeDownloadValues.length > 0
          ? activeDownloadValues.reduce((a, b) => a + b, 0) /
            activeDownloadValues.length
          : 0,
      maximum:
        maxDownloadValues.length > 0 ? Math.max(...maxDownloadValues) : 0,
    },
    upload: {
      current:
        uploadValues.length > 0 ? uploadValues[uploadValues.length - 1] : 0,
      average:
        activeUploadValues.length > 0
          ? activeUploadValues.reduce((a, b) => a + b, 0) /
            activeUploadValues.length
          : 0,
      maximum: maxUploadValues.length > 0 ? Math.max(...maxUploadValues) : 0,
    },
  };

  const rangeDomain: [number, number] | undefined = range
    ? [new Date(range.start).getTime(), new Date(range.end).getTime()]
    : getPeriodDomain(period);
  const axisTicks = getAxisTicks(rangeDomain);

  const formatXAxisTick = (time: string | number) => {
    const date = new Date(time);
    if (range?.bucket === "month") {
      return date.toLocaleDateString("en-US", { month: "short" });
    }
    if (isCustomRange) {
      return date.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
      });
    }
    return date.toLocaleTimeString("en-US", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const customRangeLabel = range
    ? `${formatXAxisTick(range.start)} - ${formatXAxisTick(range.end)}`
    : undefined;

  const formatTooltipLabel = (label: unknown) => {
    if (
      typeof label !== "string" &&
      typeof label !== "number" &&
      !(label instanceof Date)
    ) {
      return "";
    }
    return new Date(label).toLocaleString();
  };

  return (
    <Card
      size="small"
      style={{ height: "100%" }}
      bodyStyle={{ padding: "12px" }}
    >
      <div style={{ marginBottom: 8 }}>
        <div
          style={{
            fontSize: 13,
            fontWeight: 500,
            display: "flex",
            alignItems: "center",
            gap: 8,
          }}
        >
          <span>
            gpon_{ont.slot || 1}/{ont.portId}:{ont.ontId} - {ont.serialNumber} (
            {ont.deviceType || "F680V9"})
          </span>
          <StatusTag status={ont.status} />
        </div>
        <div style={{ fontSize: 11, color: "#666" }}>{ont.name}</div>
        <div style={{ fontSize: 10, color: "#999" }}>ONU-{ont.ontId}</div>
        {isCustomRange && (
          <div
            role="status"
            aria-label="Custom date range indicator"
            style={{ fontSize: 10, color: "#faad14", marginTop: 4 }}
          >
            Custom range: {customRangeLabel}
          </div>
        )}
      </div>

      {isLoading ? (
        <div style={{ display: "flex", justifyContent: "center", padding: 40 }}>
          <Spin />
        </div>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart
              data={chartData}
              margin={{ top: 10, right: 10, left: 0, bottom: 0 }}
            >
              <defs>
                <linearGradient
                  id={`colorDownload-${ont.id}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="5%" stopColor="#ff69b4" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#ff69b4" stopOpacity={0} />
                </linearGradient>
                <linearGradient
                  id={`colorUpload-${ont.id}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="5%" stopColor="#4169e1" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#4169e1" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis
                dataKey="time"
                type="number"
                domain={rangeDomain}
                ticks={axisTicks}
                allowDataOverflow
                scale="time"
                tick={{ fontSize: 10 }}
                tickLine={false}
                tickFormatter={formatXAxisTick}
              />
              <YAxis
                tick={{ fontSize: 10 }}
                tickLine={false}
                tickFormatter={(value) =>
                  value < 1 ? `${(value * 1000).toFixed(0)}K` : value.toFixed(1)
                }
              />
              <Tooltip
                contentStyle={{ fontSize: 11 }}
                formatter={(value) => formatSpeed(Number(value))}
                labelFormatter={formatTooltipLabel}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} iconType="line" />
              <Area
                type="monotone"
                dataKey="download"
                stroke="#ff69b4"
                fillOpacity={1}
                fill={`url(#colorDownload-${ont.id})`}
                name="Download"
                strokeWidth={2}
              />
              <Area
                type="monotone"
                dataKey="upload"
                stroke="#4169e1"
                fillOpacity={1}
                fill={`url(#colorUpload-${ont.id})`}
                name="Upload"
                strokeWidth={2}
              />
            </AreaChart>
          </ResponsiveContainer>

          <div style={{ marginTop: 12, fontSize: 11, lineHeight: 1.6 }}>
            <div style={{ display: "flex", gap: 16, marginBottom: 4 }}>
              <span style={{ color: "#ff69b4", fontWeight: 500 }}>
                ■ Download
              </span>
              <span>Current: {formatSpeed(stats.download.current)}</span>
              <span>Average: {formatSpeed(stats.download.average)}</span>
              <span>Maximum: {formatSpeed(stats.download.maximum)}</span>
              {stats.percentile95Download !== undefined && (
                <span>95th: {formatSpeed(stats.percentile95Download)}</span>
              )}
              <span>Total: {formatBytes(usage?.downloadBytes)}</span>
            </div>
            <div style={{ display: "flex", gap: 16 }}>
              <span style={{ color: "#4169e1", fontWeight: 500 }}>
                ■ Upload
              </span>
              <span style={{ marginLeft: 8 }}>
                Current: {formatSpeed(stats.upload.current)}
              </span>
              <span>Average: {formatSpeed(stats.upload.average)}</span>
              <span>Maximum: {formatSpeed(stats.upload.maximum)}</span>
              {stats.percentile95Upload !== undefined && (
                <span>95th: {formatSpeed(stats.percentile95Upload)}</span>
              )}
              <span>Total: {formatBytes(usage?.uploadBytes)}</span>
            </div>
          </div>
        </>
      )}
    </Card>
  );
}
