import type { OltPort, ChassisEntity } from "@/domain/entities";

// entPhysicalClass values from ENTITY-MIB. The C300 reports its shelf as
// chassis, line and control cards as module, and power supplies as powerSupply.
const ENTITY_CLASS_CHASSIS = 3;
const ENTITY_CLASS_MODULE = 3;
const ENTITY_CLASS_POWER = 6;

export interface SlotPorts {
  slot: number;
  ports: OltPort[];
}

// Ports arrive as one flat inventory. Both the uplink and PON views are drawn
// per card, so they are grouped by the slot parsed out of the interface name.
export function groupPortsBySlot(
  ports: OltPort[],
  kind: OltPort["kind"],
): SlotPorts[] {
  const bySlot = new Map<number, OltPort[]>();

  for (const port of ports) {
    if (port.kind !== kind) continue;
    const existing = bySlot.get(port.slot);
    if (existing) {
      existing.push(port);
    } else {
      bySlot.set(port.slot, [port]);
    }
  }

  return Array.from(bySlot.entries())
    .map(([slot, slotPorts]) => ({
      slot,
      ports: [...slotPorts].sort((a, b) => a.port - b.port),
    }))
    .sort((a, b) => a.slot - b.slot);
}

// The OLT reports uptime as seconds since its last restart. Days matter to an
// operator judging stability; anything below a minute does not.
export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "unknown";

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  parts.push(`${minutes}m`);

  return parts.join(" ");
}

// sysDescr is a full copyright banner. The header has room for the model and
// the software version, which are the two things an operator reads off it.
// An OLT the poll has not read yet has no banner at all, and the caller needs
// something printable: an empty string renders as 0 in a Statistic.
export function summariseModel(description: string): string {
  const banner = description.trim();
  if (banner === "") return "unknown";

  const match = banner.match(/^(\S+)\s+Version\s+(\S+)/i);
  return match ? `${match[1]} ${match[2]}` : banner.split(",")[0].trim();
}

export function isPowerEntity(entity: ChassisEntity): boolean {
  return entity.class === ENTITY_CLASS_POWER;
}

export function isCardEntity(entity: ChassisEntity): boolean {
  return (
    entity.class === ENTITY_CLASS_MODULE ||
    entity.class === ENTITY_CLASS_CHASSIS
  );
}
