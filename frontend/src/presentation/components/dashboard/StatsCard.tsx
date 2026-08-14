import { Card, Statistic } from 'antd';
import type { ReactNode } from 'react';

interface StatsCardProps {
  title: string;
  value: number;
  icon: ReactNode;
  loading?: boolean;
}

export function StatsCard({ title, value, icon, loading }: StatsCardProps) {
  return (
    <Card>
      <Statistic
        title={title}
        value={value}
        prefix={icon}
        loading={loading}
      />
    </Card>
  );
}
