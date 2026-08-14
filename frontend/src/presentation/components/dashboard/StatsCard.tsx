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
    <Card loading={loading} className="border-slate-200">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <p className="text-sm text-slate-600 mb-1">{title}</p>
          <h3 className="text-2xl font-semibold text-slate-900">
            {value.toLocaleString()}
          </h3>
        </div>
        <div className="text-emerald-600 opacity-80">
          {icon}
        </div>
      </div>
    </Card>
  );
}
