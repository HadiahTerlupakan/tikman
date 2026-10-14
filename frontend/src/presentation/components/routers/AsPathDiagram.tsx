import { theme, Typography } from "antd";
import type { PathGraph, PathNode } from "./asPathGraph";
import { ROUTER_NODE } from "./asPathGraph";

const COLUMN_WIDTH = 210;
const NODE_WIDTH = 160;
const NODE_HEIGHT = 46;
const ROW_STEP = 64;
const PADDING = 12;
const MAX_STROKE = 9;
const MIN_STROKE = 1.5;
const NAME_CHARS = 20;

interface Props {
  graph: PathGraph;
  /** AS number (as a string key) to name. */
  names: Record<string, string>;
  routerLabel: string;
  selectedAsn?: number | null;
  onSelect?: (node: PathNode) => void;
}

interface Placed {
  node: PathNode;
  x: number;
  y: number;
}

function place(graph: PathGraph): Map<string, Placed> {
  const rows = new Map<number, PathNode[]>();
  for (const n of graph.nodes) {
    rows.set(n.column, [...(rows.get(n.column) ?? []), n]);
  }
  const tallest = Math.max(...[...rows.values()].map((r) => r.length));
  const placed = new Map<string, Placed>();
  for (const [column, nodes] of rows) {
    const offset = ((tallest - nodes.length) * ROW_STEP) / 2;
    nodes.forEach((node, i) => {
      placed.set(node.id, {
        node,
        x: PADDING + column * COLUMN_WIDTH,
        y: PADDING + offset + i * ROW_STEP,
      });
    });
  }
  return placed;
}

function shorten(name: string): string {
  return name.length > NAME_CHARS ? `${name.slice(0, NAME_CHARS - 1)}…` : name;
}

function nodeTitle(node: PathNode, names: Record<string, string>): string {
  const lines = [
    node.asn === null
      ? "Router ini"
      : `AS${node.asn} ${names[String(node.asn)] ?? ""}`.trim(),
  ];
  if (node.peers.length > 0) lines.push(`Sesi: ${node.peers.join(", ")}`);
  lines.push(`${node.prefixes.length} prefix`);
  if (!node.active) lines.push("Hanya di jalur alternatif");
  return lines.join("\n");
}

export function AsPathDiagram({
  graph,
  names,
  routerLabel,
  selectedAsn = null,
  onSelect,
}: Props) {
  const { token } = theme.useToken();
  if (graph.nodes.length === 0) {
    return (
      <Typography.Text type="secondary">
        Belum ada jalur untuk digambar.
      </Typography.Text>
    );
  }
  const placed = place(graph);
  const tallest = Math.max(
    ...[...placed.values()].map((p) => p.y + NODE_HEIGHT),
  );
  const width = PADDING * 2 + (graph.columns - 1) * COLUMN_WIDTH + NODE_WIDTH;
  const height = tallest + PADDING;
  const maxBytes = Math.max(...graph.edges.map((e) => e.bytes));
  const stroke = (bytes: number) =>
    maxBytes === 0
      ? 2
      : MIN_STROKE + Math.sqrt(bytes / maxBytes) * (MAX_STROKE - MIN_STROKE);

  return (
    <div style={{ overflowX: "auto" }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="Bagan jalur AS"
        style={{ display: "block", fontFamily: token.fontFamily }}
      >
        {graph.edges.map((e) => {
          const from = placed.get(e.from);
          const to = placed.get(e.to);
          if (!from || !to) return null;
          const x1 = from.x + NODE_WIDTH;
          const y1 = from.y + NODE_HEIGHT / 2;
          const x2 = to.x;
          const y2 = to.y + NODE_HEIGHT / 2;
          const mid = (x1 + x2) / 2;
          return (
            <path
              key={`${e.from}>${e.to}`}
              data-edge
              data-active={e.active ? "true" : "false"}
              d={`M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}`}
              fill="none"
              stroke={e.active ? token.colorPrimary : token.colorTextQuaternary}
              strokeWidth={stroke(e.bytes)}
              strokeDasharray={e.active ? undefined : "6 4"}
              strokeLinecap="round"
            >
              <title>{`${e.prefixes.length} prefix`}</title>
            </path>
          );
        })}
        {[...placed.values()].map(({ node, x, y }) => {
          const selected = node.asn !== null && node.asn === selectedAsn;
          const fill =
            node.id === ROUTER_NODE
              ? token.colorPrimaryBg
              : node.origin
                ? token.colorSuccessBg
                : token.colorBgContainer;
          return (
            <g
              key={node.id}
              transform={`translate(${x},${y})`}
              onClick={onSelect ? () => onSelect(node) : undefined}
              style={{ cursor: onSelect ? "pointer" : "default" }}
              opacity={node.active ? 1 : 0.6}
            >
              <title>{nodeTitle(node, names)}</title>
              <rect
                width={NODE_WIDTH}
                height={NODE_HEIGHT}
                rx={8}
                fill={fill}
                stroke={selected ? token.colorPrimary : token.colorBorder}
                strokeWidth={selected ? 2.5 : 1}
                strokeDasharray={node.active ? undefined : "4 3"}
              />
              <text
                x={10}
                y={19}
                fontSize={13}
                fontWeight={600}
                fill={token.colorText}
              >
                {node.asn === null ? routerLabel : `AS${node.asn}`}
              </text>
              <text x={10} y={36} fontSize={11} fill={token.colorTextSecondary}>
                {node.asn === null
                  ? `${node.prefixes.length} prefix`
                  : shorten(names[String(node.asn)] ?? node.peers.join(", "))}
              </text>
              {node.prepends > 0 && (
                <text
                  x={NODE_WIDTH - 8}
                  y={14}
                  fontSize={10}
                  textAnchor="end"
                  fill={token.colorWarningText}
                >
                  {`+${node.prepends} prepend`}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
