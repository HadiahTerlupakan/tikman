import { Empty, theme } from "antd";
import type { PonHealth } from "@/domain/entities";
import { layoutPonTree, type LaidOutNode } from "./ponLayout";

interface PonTopologyProps {
  health: PonHealth;
  onSelectPon: (slot: number, port: number) => void;
}

/**
 * PonTopology draws the pruned tree: OLT, then only the cards and ports in
 * trouble, then the subscribers worst hit on each.
 *
 * Severity is scored against the worst port drawn rather than a fixed scale, so
 * the colours say where the trouble is concentrated on this chassis instead of
 * claiming an absolute standard this network never agreed to.
 */
export function PonTopology({ health, onSelectPon }: PonTopologyProps) {
  const { token } = theme.useToken();
  const { nodes, edges, width, height } = layoutPonTree(health);

  if (nodes.length === 0) {
    return (
      <Empty
        description={`Tidak ada PON bermasalah di ${health.oltName} pada rentang ini`}
      />
    );
  }

  const fill = (node: LaidOutNode) => {
    if (node.severity > 0.66) return token.colorErrorBg;
    if (node.severity > 0.33) return token.colorWarningBg;
    return token.colorBgContainer;
  };

  const stroke = (node: LaidOutNode) => {
    if (node.severity > 0.66) return token.colorError;
    if (node.severity > 0.33) return token.colorWarning;
    return token.colorBorderSecondary;
  };

  return (
    <div style={{ overflowX: "auto" }}>
      <svg
        width={width + 8}
        height={height + 8}
        role="img"
        aria-label={`Topologi PON bermasalah ${health.oltName}`}
      >
        {edges.map((edge) => (
          <path
            key={edge.id}
            d={edge.path}
            fill="none"
            stroke={token.colorBorderSecondary}
            strokeWidth={1}
          />
        ))}
        {nodes.map((node) => (
          <g
            key={node.id}
            onClick={() => {
              if (node.kind !== "pon") return;
              const [, slot, port] = node.id.split("-");
              onSelectPon(Number(slot), Number(port));
            }}
            style={{ cursor: node.kind === "pon" ? "pointer" : "default" }}
          >
            <rect
              x={node.x}
              y={node.y}
              width={node.width}
              height={node.height}
              rx={8}
              fill={fill(node)}
              stroke={stroke(node)}
            />
            <text
              x={node.x + 12}
              y={node.y + 21}
              fill={token.colorText}
              fontSize={13}
              fontWeight={600}
            >
              {node.label}
            </text>
            <text
              x={node.x + 12}
              y={node.y + 39}
              fill={token.colorTextSecondary}
              fontSize={11}
            >
              {node.detail}
            </text>
          </g>
        ))}
      </svg>
      <div
        style={{ fontSize: 11, color: token.colorTextSecondary, marginTop: 8 }}
      >
        Ditampilkan bila kehilangan layanan &gt;{" "}
        {Math.round(health.outageThreshold * 100)}% rentang, atau trap/ONT di
        atas {health.trapThreshold} dan lima kali median OLT ini (
        {health.medianTrapPerOnt}).
      </div>
    </div>
  );
}
