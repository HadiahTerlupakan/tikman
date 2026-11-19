import type { MikrotikRouter } from "@/domain/entities";

export type RackSlot =
  | { kind: "router"; router: MikrotikRouter; u: number }
  | { kind: "blank"; u: number };

const byName = (a: MikrotikRouter, b: MikrotikRouter) =>
  a.name.localeCompare(b.name);

/** Lays routers out bottom-up by U; a U claimed twice keeps its first router by name. */
export function rackSlots(routers: MikrotikRouter[]): RackSlot[] {
  const placed = new Map<number, MikrotikRouter>();
  const loose: MikrotikRouter[] = [];
  const clashed: MikrotikRouter[] = [];
  for (const r of [...routers].sort(byName)) {
    if (r.rackPosition === null) loose.push(r);
    else if (placed.has(r.rackPosition)) clashed.push(r);
    else placed.set(r.rackPosition, r);
  }

  const slots: RackSlot[] = [];
  const top = Math.max(0, ...placed.keys());
  for (let u = 1; u <= top; u++) {
    const router = placed.get(u);
    slots.push(router ? { kind: "router", router, u } : { kind: "blank", u });
  }
  for (const router of [...loose, ...clashed]) {
    slots.push({ kind: "router", router, u: slots.length + 1 });
  }
  return slots;
}
