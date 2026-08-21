import { Card, Spin } from "antd";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts";
import { Ont } from "@/domain/entities/Ont";
import { useOntTrafficTimeSeries } from "@/application/hooks/useOntMetrics";

interface OntTrafficCardProps {
  ont: Ont;
  period: string;
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

export function OntTrafficCard({ ont, period }: OntTrafficCardProps) {
  const { data: timeSeries, isLoading } = useOntTrafficTimeSeries(ont.id, period);

  const chartData = timeSeries?.map((point) => ({
    time: new Date(point.time).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' }),
    download: point.txMbps || 0,
    upload: point.rxMbps || 0,
  })) || [];

  const downloadValues = timeSeries?.map(p => p.txMbps || 0) || [];
  const uploadValues = timeSeries?.map(p => p.rxMbps || 0) || [];

  const stats = {
    download: {
      current: downloadValues.length > 0 ? downloadValues[downloadValues.length - 1] : 0,
      average: downloadValues.length > 0 ? downloadValues.reduce((a, b) => a + b, 0) / downloadValues.length : 0,
      maximum: downloadValues.length > 0 ? Math.max(...downloadValues) : 0,
    },
    upload: {
      current: uploadValues.length > 0 ? uploadValues[uploadValues.length - 1] : 0,
      average: uploadValues.length > 0 ? uploadValues.reduce((a, b) => a + b, 0) / uploadValues.length : 0,
      maximum: uploadValues.length > 0 ? Math.max(...uploadValues) : 0,
    },
  };

  return (
    <Card
      size="small"
      style={{ height: '100%' }}
      bodyStyle={{ padding: '12px' }}
    >
      <div style={{ marginBottom: 8 }}>
        <div style={{ fontSize: 13, fontWeight: 500 }}>
          gpon_{ont.slot || 1}/{ont.portId}:{ont.ontId} - {ont.serialNumber} ({ont.deviceType || 'F680V9'})
        </div>
        <div style={{ fontSize: 11, color: '#666' }}>
          {ont.name}
        </div>
        <div style={{ fontSize: 10, color: '#999' }}>
          ONU-{ont.ontId}
        </div>
      </div>

      {isLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}>
          <Spin />
        </div>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id={`colorDownload-${ont.id}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#ff69b4" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#ff69b4" stopOpacity={0} />
                </linearGradient>
                <linearGradient id={`colorUpload-${ont.id}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#4169e1" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#4169e1" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis 
                dataKey="time" 
                tick={{ fontSize: 10 }} 
                tickLine={false}
              />
              <YAxis 
                tick={{ fontSize: 10 }} 
                tickLine={false}
                tickFormatter={(value) => value < 1 ? `${(value * 1000).toFixed(0)}K` : value.toFixed(1)}
              />
              <Tooltip 
                contentStyle={{ fontSize: 11 }}
                formatter={(value) => formatSpeed(Number(value))}
              />
              <Legend 
                wrapperStyle={{ fontSize: 11 }}
                iconType="line"
              />
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
            <div style={{ display: 'flex', gap: 16, marginBottom: 4 }}>
              <span style={{ color: '#ff69b4', fontWeight: 500 }}>■ Download</span>
              <span>Current: {formatSpeed(stats.download.current)}</span>
              <span>Average: {formatSpeed(stats.download.average)}</span>
              <span>Maximum: {formatSpeed(stats.download.maximum)}</span>
            </div>
            <div style={{ display: 'flex', gap: 16 }}>
              <span style={{ color: '#4169e1', fontWeight: 500 }}>■ Upload</span>
              <span style={{ marginLeft: 8 }}>Current: {formatSpeed(stats.upload.current)}</span>
              <span>Average: {formatSpeed(stats.upload.average)}</span>
              <span>Maximum: {formatSpeed(stats.upload.maximum)}</span>
            </div>
          </div>
        </>
      )}
    </Card>
  );
}
