/** One candidate route as the diagram reads it. */
export interface PathSource {
  prefix: string;
  /** The BGP session the route came in on. */
  peer: string;
  /** RouterOS's bgp.as-path, comma or space separated, AS sets in braces. */
  asPath: string;
  active: boolean;
  /** Weight for the line width; 0 when unknown. */
  bytes: number;
}

export interface PathNode {
  id: string;
  column: number;
  /** null for the router itself. */
  asn: number | null;
  bytes: number;
  active: boolean;
  /** The last AS of at least one path: where the traffic ends. */
  origin: boolean;
  /** Sessions whose paths start at this AS (first column only). */
  peers: string[];
  prefixes: string[];
  /** Highest number of times this AS repeated itself on one path. */
  prepends: number;
}

export interface PathEdge {
  from: string;
  to: string;
  bytes: number;
  active: boolean;
  prefixes: string[];
}

export interface PathGraph {
  nodes: PathNode[];
  edges: PathEdge[];
  columns: number;
}

export const ROUTER_NODE = "router";

interface Hop {
  asn: number;
  prepends: number;
}

/**
 * Splits an AS path into hops, collapsing prepends into a count. An AS set
 * ("{64496,64497}", an aggregate) is one hop, shown as its first member.
 */
export function pathHops(asPath: string): Hop[] {
  const hops: Hop[] = [];
  for (const token of asPath.match(/\{[^}]*\}|\d+/g) ?? []) {
    const first = token.match(/\d+/);
    if (!first) continue;
    const asn = Number(first[0]);
    const last = hops[hops.length - 1];
    if (last && last.asn === asn) last.prepends += 1;
    else hops.push({ asn, prepends: 0 });
  }
  return hops;
}

function pushUnique(list: string[], value: string) {
  if (!list.includes(value)) list.push(value);
}

/**
 * Lays candidate routes out as a layered graph: the router in column 0, the
 * first AS of each path in column 1, and so on. The same AS at the same depth
 * is one node, so prefixes sharing a peer share its node and line.
 */
export function buildPathGraph(paths: PathSource[]): PathGraph {
  const nodes = new Map<string, PathNode>();
  const edges = new Map<string, PathEdge>();
  const node = (id: string, column: number, asn: number | null) => {
    let n = nodes.get(id);
    if (!n) {
      n = {
        id,
        column,
        asn,
        bytes: 0,
        active: false,
        origin: false,
        peers: [],
        prefixes: [],
        prepends: 0,
      };
      nodes.set(id, n);
    }
    return n;
  };
  for (const p of paths) {
    const hops = pathHops(p.asPath);
    if (hops.length === 0) continue;
    let prev = node(ROUTER_NODE, 0, null);
    prev.bytes += p.bytes;
    prev.active ||= p.active;
    pushUnique(prev.prefixes, p.prefix);
    hops.forEach((hop, i) => {
      const n = node(`${i + 1}:${hop.asn}`, i + 1, hop.asn);
      n.bytes += p.bytes;
      n.active ||= p.active;
      n.origin ||= i === hops.length - 1;
      n.prepends = Math.max(n.prepends, hop.prepends);
      pushUnique(n.prefixes, p.prefix);
      if (i === 0) pushUnique(n.peers, p.peer);
      const key = `${prev.id}>${n.id}`;
      let e = edges.get(key);
      if (!e) {
        e = { from: prev.id, to: n.id, bytes: 0, active: false, prefixes: [] };
        edges.set(key, e);
      }
      e.bytes += p.bytes;
      e.active ||= p.active;
      pushUnique(e.prefixes, p.prefix);
      prev = n;
    });
  }
  const sorted = [...nodes.values()].sort(
    (a, b) =>
      a.column - b.column || b.bytes - a.bytes || (a.asn ?? 0) - (b.asn ?? 0),
  );
  const columns =
    sorted.length === 0 ? 0 : sorted[sorted.length - 1].column + 1;
  return { nodes: sorted, edges: [...edges.values()], columns };
}
