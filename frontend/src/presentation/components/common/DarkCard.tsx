import { Card } from 'antd';
import type { CardProps } from 'antd';
import type { ReactNode } from 'react';

interface DarkCardProps extends Omit<CardProps, 'title'> {
  title?: ReactNode;
}

export function DarkCard({ title, children, style, ...props }: DarkCardProps) {
  const titleNode = typeof title === 'string' ? (
    <span style={{ color: '#ffffff' }}>{title}</span>
  ) : title;

  return (
    <Card
      bordered={false}
      title={titleNode}
      style={{
        background: '#18181b',
        border: '1px solid #27272a',
        ...style,
      }}
      {...props}
    >
      {children}
    </Card>
  );
}
