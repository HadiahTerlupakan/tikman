import { Card } from 'antd';
import type { ReactNode } from 'react';

interface StatsCardProps {
  title: string;
  value: number;
  icon: ReactNode;
  loading?: boolean;
}

export function StatsCard({ title, value, icon, loading }: StatsCardProps) {
  return (
    <Card
      loading={loading}
      className="!border-gray-200 hover:shadow-md transition-shadow"
      styles={{ body: { padding: '20px' } }}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <p className="text-xs font-medium text-gray-600 uppercase tracking-wide mb-2">{title}</p>
          <h3 className="text-3xl font-semibold text-gray-900">
            {value.toLocaleString()}
          </h3>
        </div>
        <div className="w-10 h-10 flex items-center justify-center rounded-lg bg-emerald-50">
          <div className="text-emerald-600">
            {icon}
          </div>
        </div>
      </div>
    </Card>
  );
}
