import type { CardNode, PonHealth, PonNode } from "@/domain/entities";

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

/** A running row counter, threaded by reference through every level so a
 * child's row picks up where its earlier siblings left off. */
interface RowCounter {
  value: number;
}

/**
 * Canvas is what every level appends to. `index` mirrors `nodes` by id so an
 * edge finds its endpoints in constant time; scanning the array made laying out
 * a card-wide failure quadratic in the number of ports drawn.
 */
interface Canvas {
  nodes: LaidOutNode[];
  index: Map<string, LaidOutNode>;
  edges: LaidOutEdge[];
}

/**
 * Scale is what severity is measured against: the worst churn and the worst
 * service loss among the ports drawn.
 *
 * Both, because the server flags a port on either rule and neither sees the
 * other's fault. Scoring on traps alone would leave a port whose subscribers
 * lose a tenth of the day drawn in the neutral fill.
 */
interface Scale {
  worstTrap: number;
  worstOutage: number;
}

/** severityOf reads a port on whichever of the two measures it fails hardest. */
function severityOf(pon: PonNode, scale: Scale): number {
  const churn = pon.trapPerOnt / scale.worstTrap;
  const outage =
    scale.worstOutage > 0 ? pon.outageShare / scale.worstOutage : 0;
  return Math.max(churn, outage);
}

/** curve draws the S-bend between two columns, as in a pipeline diagram. */
function curve(x1: number, y1: number, x2: number, y2: number): string {
  const mid = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`;
}

/** centreOf puts a parent level with its children, or on its own row if none. */
function centreOf(childYs: number[], fallbackRow: number): number {
  if (childYs.length === 0) return fallbackRow * (NODE_HEIGHT + GAP);
  return (childYs[0] + childYs[childYs.length - 1]) / 2;
}

function pushNode(
  canvas: Canvas,
  kind: LaidOutNode["kind"],
  id: string,
  label: string,
  detail: string,
  y: number,
  severity: number,
): void {
  const level = ["olt", "card", "pon", "ont"].indexOf(kind);
  const node: LaidOutNode = {
    id,
    kind,
    label,
    detail,
    x: COLUMN_X[level],
    y,
    width: NODE_WIDTH[level],
    height: NODE_HEIGHT,
    severity,
  };
  canvas.nodes.push(node);
  canvas.index.set(id, node);
}

function pushEdge(canvas: Canvas, from: string, to: string): void {
  const a = canvas.index.get(from);
  const b = canvas.index.get(to);
  if (!a || !b) return;
  canvas.edges.push({
    id: `${from}->${to}`,
    from,
    to,
    path: curve(a.x + a.width, a.y + a.height / 2, b.x, b.y + b.height / 2),
  });
}

/** layoutSubscribers places one row per worst-hit subscriber on a PON. */
function layoutSubscribers(
  pon: PonNode,
  row: RowCounter,
  canvas: Canvas,
): number[] {
  const centres: number[] = [];
  const worstOfPon = Math.max(...pon.worst.map((w) => w.trapCount), 1);

  for (const ont of pon.worst) {
    const y = row.value * (NODE_HEIGHT + GAP);
    pushNode(
      canvas,
      "ont",
      `ont-${ont.ontId}`,
      ont.label,
      `${ont.trapCount.toLocaleString("id-ID")} trap · ${ont.downMinutes} mnt`,
      y,
      ont.trapCount / worstOfPon,
    );
    centres.push(y);
    row.value += 1;
  }
  return centres;
}

/** layoutPons places every troubled port on a card, centred on its subscribers. */
function layoutPons(
  card: CardNode,
  scale: Scale,
  row: RowCounter,
  canvas: Canvas,
): number[] {
  const centres: number[] = [];

  for (const pon of card.pons) {
    const ponId = `pon-${card.slot}-${pon.port}`;
    const subscriberCentres = layoutSubscribers(pon, row, canvas);
    const ponY = centreOf(subscriberCentres, row.value);
    pushNode(
      canvas,
      "pon",
      ponId,
      `PON ${pon.port}`,
      `${pon.trapPerOnt} trap/ONT · ${Math.round(pon.outageShare * 100)}% mati`,
      ponY,
      severityOf(pon, scale),
    );
    for (const ont of pon.worst) pushEdge(canvas, ponId, `ont-${ont.ontId}`);
    centres.push(ponY);
  }
  return centres;
}

/** layoutCards places every troubled card, centred on its troubled ports. */
function layoutCards(
  health: PonHealth,
  scale: Scale,
  row: RowCounter,
  canvas: Canvas,
): number[] {
  const centres: number[] = [];

  for (const card of health.cards) {
    const cardId = `card-${card.slot}`;
    const ponCentres = layoutPons(card, scale, row, canvas);
    const cardY = centreOf(ponCentres, row.value);
    pushNode(
      canvas,
      "card",
      cardId,
      `Kartu ${card.slot}`,
      `${card.ponCount} PON`,
      cardY,
      0,
    );
    for (const pon of card.pons)
      pushEdge(canvas, cardId, `pon-${card.slot}-${pon.port}`);
    centres.push(cardY);
  }
  return centres;
}

/**
 * layoutPonTree turns the pruned tree into placed boxes and connectors.
 *
 * Depth is fixed at four, so this is column arithmetic rather than graph
 * layout: each leaf takes the next row, and a parent centres on its children.
 * Kept apart from the drawing so the arithmetic can be tested directly —
 * testing placement through the DOM only tests React. Each level's placement
 * is its own helper above so this function reads as the sequence it is:
 * subscribers, then ports, then cards, then the OLT they sit under.
 */
export function layoutPonTree(health: PonHealth): PonLayout {
  const canvas: Canvas = { nodes: [], index: new Map(), edges: [] };
  const { nodes, edges } = canvas;
  if (health.cards.length === 0) return { nodes, edges, width: 0, height: 0 };

  const pons = health.cards.flatMap((c) => c.pons);
  const scale: Scale = {
    worstTrap: Math.max(...pons.map((p) => p.trapPerOnt), 1),
    worstOutage: Math.max(...pons.map((p) => p.outageShare), 0),
  };
  const row: RowCounter = { value: 0 };
  const cardCentres = layoutCards(health, scale, row, canvas);

  const oltY = centreOf(cardCentres, row.value);
  pushNode(
    canvas,
    "olt",
    "olt",
    health.oltName,
    `median ${health.medianTrapPerOnt} trap/ONT`,
    oltY,
    0,
  );
  for (const card of health.cards) pushEdge(canvas, "olt", `card-${card.slot}`);

  // Built leaf-first (an ONT's row has to exist before its PON can centre on
  // it), but the tree reads root-first, so sort into level order for return.
  const levelOrder: LaidOutNode["kind"][] = ["olt", "card", "pon", "ont"];
  nodes.sort((a, b) => levelOrder.indexOf(a.kind) - levelOrder.indexOf(b.kind));

  const height = Math.max(...nodes.map((n) => n.y + n.height));
  const width = Math.max(...nodes.map((n) => n.x + n.width));
  return { nodes, edges, width, height };
}
