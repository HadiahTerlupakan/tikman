import type { RouteAdvice, RouteLookup } from "@/domain/entities";
import type { PathGraph, PathSource } from "./asPathGraph";

/** The busiest destinations' routes, active ones first, as diagram input. */
export function advicePaths(
  rows: RouteAdvice[],
  withAlternatives: boolean,
): PathSource[] {
  const paths: PathSource[] = [];
  for (const row of rows) {
    const bytes = row.downloadBytes + row.uploadBytes;
    if (row.active) {
      paths.push({
        prefix: row.prefix,
        peer: row.active.peer,
        asPath: row.active.asPath,
        active: true,
        bytes,
      });
    }
    if (!withAlternatives) continue;
    for (const alt of row.alternatives ?? []) {
      paths.push({
        prefix: row.prefix,
        peer: alt.peer,
        asPath: alt.asPath,
        active: false,
        bytes,
      });
    }
  }
  return paths;
}

/** A live lookup's candidates; no traffic is known, so every line is even. */
export function lookupPaths(lookup: RouteLookup): PathSource[] {
  return lookup.routes.map((r) => ({
    prefix: lookup.prefix,
    peer: r.session,
    asPath: r.asPath,
    active: r.active,
    bytes: 0,
  }));
}

/** The AS numbers on a graph, sorted, for one names request. */
export function graphAsns(graph: PathGraph): number[] {
  const asns = new Set<number>();
  for (const n of graph.nodes) if (n.asn !== null) asns.add(n.asn);
  return [...asns].sort((a, b) => a - b);
}

/** Origin names the advice already carries, under the dataset's names. */
export function adviceNames(rows: RouteAdvice[]): Record<string, string> {
  const names: Record<string, string> = {};
  for (const row of rows) if (row.name) names[String(row.asn)] = row.name;
  return names;
}
