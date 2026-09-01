import type { PonHealth } from "@/domain/entities";

const COLUMN_X = [0, 200, 400, 640];
const NODE_WIDTH = [150, 150, 200, 240];
const NODE_HEIGHT = 52;
const GAP = 14;

export interface LaidOutNode {
  id: string;
  kind: "olt" | "card" | "pon" | "ont";
  label: string;
  detail: string;
  x: number;
  y: number;
  width: number;
  height: number;
  severity: number;
}

export interface LaidOutEdge {
  id: string;
  from: string;
  to: string;
  path: string;
}

export interface PonLayout {
  nodes: LaidOutNode[];
  edges: LaidOutEdge[];
  width: number;
  height: number;
}

/** curve draws the S-bend between two columns, as in a pipeline diagram. */
function curve(x1: number, y1: number, x2: number, y2: number): string {
  const mid = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`;
}

/**
 * layoutPonTree turns the pruned tree into placed boxes and connectors.
 *
 * Depth is fixed at four, so this is column arithmetic rather than graph
 * layout: each leaf takes the next row, and a parent centres on its children.
 * Kept apart from the drawing so the arithmetic can be tested directly —
 * testing placement through the DOM only tests React.
 */
export function layoutPonTree(health: PonHealth): PonLayout {
  const nodes: LaidOutNode[] = [];
  const edges: LaidOutEdge[] = [];
  if (health.cards.length === 0) return { nodes, edges, width: 0, height: 0 };

  const worstTrap = Math.max(
    ...health.cards.flatMap((c) => c.pons.map((p) => p.trapPerOnt)),
    1,
  );
  let row = 0;

  const push = (
    kind: LaidOutNode["kind"],
    id: string,
    label: string,
    detail: string,
    y: number,
    severity: number,
  ) => {
    const level = ["olt", "card", "pon", "ont"].indexOf(kind);
    nodes.push({
      id,
      kind,
      label,
      detail,
      x: COLUMN_X[level],
      y,
      width: NODE_WIDTH[level],
      height: NODE_HEIGHT,
      severity,
    });
  };

  const link = (from: string, to: string) => {
    const a = nodes.find((n) => n.id === from);
    const b = nodes.find((n) => n.id === to);
    if (!a || !b) return;
    edges.push({
      id: `${from}->${to}`,
      from,
      to,
      path: curve(a.x + a.width, a.y + a.height / 2, b.x, b.y + b.height / 2),
    });
  };

  const cardCentres: number[] = [];

  for (const card of health.cards) {
    const ponCentres: number[] = [];
    const cardId = `card-${card.slot}`;

    for (const pon of card.pons) {
      const ontCentres: number[] = [];
      const ponId = `pon-${card.slot}-${pon.port}`;

      for (const ont of pon.worst) {
        const y = row * (NODE_HEIGHT + GAP);
        push(
          "ont",
          `ont-${ont.ontId}`,
          ont.label,
          `${ont.trapCount.toLocaleString("id-ID")} trap · ${ont.downMinutes} mnt`,
          y,
          ont.trapCount / Math.max(...pon.worst.map((w) => w.trapCount), 1),
        );
        ontCentres.push(y);
        row += 1;
      }

      const ponY = centreOf(ontCentres, row);
      push(
        "pon",
        ponId,
        `PON ${pon.port}`,
        `${pon.trapPerOnt} trap/ONT · ${Math.round(pon.outageShare * 100)}% mati`,
        ponY,
        pon.trapPerOnt / worstTrap,
      );
      ponCentres.push(ponY);
      for (const ont of pon.worst) link(ponId, `ont-${ont.ontId}`);
    }

    const cardY = centreOf(ponCentres, row);
    push(
      "card",
      cardId,
      `Kartu ${card.slot}`,
      `${card.ponCount} PON`,
      cardY,
      0,
    );
    cardCentres.push(cardY);
    for (const pon of card.pons) link(cardId, `pon-${card.slot}-${pon.port}`);
  }

  const oltY = centreOf(cardCentres, row);
  push(
    "olt",
    "olt",
    health.oltName,
    `median ${health.medianTrapPerOnt} trap/ONT`,
    oltY,
    0,
  );
  for (const card of health.cards) link("olt", `card-${card.slot}`);

  // Built leaf-first (an ONT's row has to exist before its PON can centre on
  // it), but the tree reads root-first, so sort into level order for return.
  const levelOrder: LaidOutNode["kind"][] = ["olt", "card", "pon", "ont"];
  nodes.sort((a, b) => levelOrder.indexOf(a.kind) - levelOrder.indexOf(b.kind));

  const height = Math.max(...nodes.map((n) => n.y + n.height));
  const width = Math.max(...nodes.map((n) => n.x + n.width));
  return { nodes, edges, width, height };
}

/** centreOf puts a parent level with its children, or on its own row if none. */
function centreOf(childYs: number[], fallbackRow: number): number {
  if (childYs.length === 0) return fallbackRow * (NODE_HEIGHT + GAP);
  return (childYs[0] + childYs[childYs.length - 1]) / 2;
}
