import { Ont } from "@/domain/entities/Ont";

/** Filters ONTs by serial number or name, case-insensitive. */
export function filterOntsByQuery(onts: Ont[], query: string): Ont[] {
  const q = query.trim().toLowerCase();
  if (!q) return onts;
  return onts.filter(
    (ont) =>
      ont.serialNumber.toLowerCase().includes(q) ||
      ont.name.toLowerCase().includes(q),
  );
}
