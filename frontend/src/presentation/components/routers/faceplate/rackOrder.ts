/** Anything that sits in the rack: a router or a switch. */
interface Rackable {
  name: string;
  rackPosition: number | null;
}

export type RackSlot<T extends Rackable> =
  | { kind: "unit"; item: T; u: number }
  | { kind: "blank"; u: number };

const byName = (a: Rackable, b: Rackable) => a.name.localeCompare(b.name);

/** Lays units out from U1 at the top down; a U claimed twice keeps its first unit by name. */
export function rackSlots<T extends Rackable>(items: T[]): RackSlot<T>[] {
  const placed = new Map<number, T>();
  const loose: T[] = [];
  const clashed: T[] = [];
  for (const r of [...items].sort(byName)) {
    if (r.rackPosition === null) loose.push(r);
    else if (placed.has(r.rackPosition)) clashed.push(r);
    else placed.set(r.rackPosition, r);
  }

  const slots: RackSlot<T>[] = [];
  const top = Math.max(0, ...placed.keys());
  for (let u = 1; u <= top; u++) {
    const item = placed.get(u);
    slots.push(item ? { kind: "unit", item, u } : { kind: "blank", u });
  }
  for (const item of [...loose, ...clashed]) {
    slots.push({ kind: "unit", item, u: slots.length + 1 });
  }
  return slots;
}
