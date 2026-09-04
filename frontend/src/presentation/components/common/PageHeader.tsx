import { Typography } from "antd";
import type { ReactNode } from "react";

const { Title, Text } = Typography;

interface PageHeaderProps {
  title: string;
  description?: string;
  extra?: ReactNode;
}

export function PageHeader({ title, description, extra }: PageHeaderProps) {
  return (
    <div
      style={{
        marginBottom: 24,
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        // Wrapping, because the actions are a row of buttons with a real
        // minimum width: on a phone they won the space outright and squeezed
        // the title column to nothing, breaking the heading one letter per
        // line. With room to spare nothing here changes.
        flexWrap: "wrap",
        gap: 16,
      }}
    >
      <div style={{ minWidth: 0 }}>
        <Title
          level={4}
          style={{
            margin: 0,
            marginBottom: description ? 8 : 0,
            color: "#ffffff",
          }}
        >
          {title}
        </Title>
        {description && <Text style={{ color: "#a1a1aa" }}>{description}</Text>}
      </div>
      {extra && <div>{extra}</div>}
    </div>
  );
}
