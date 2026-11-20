import type { SwitchInterface } from "@/domain/entities";

export type VlanMembership = "T" | "U" | "";

const RANGE = /^(\d+)(?:-(\d+))?$/;

/** "10,20-22" as a set of VLAN ids. */
export function parseVlanList(list: string): Set<number> {
  const vids = new Set<number>();
  for (const part of list.split(",")) {
    const range = RANGE.exec(part.trim());
    if (!range) continue;
    const from = Number(range[1]);
    const to = Number(range[2] ?? range[1]);
    for (let vid = from; vid <= to; vid++) vids.add(vid);
  }
  return vids;
}

// A CE lists a hybrid port's tagged and untagged VLANs separately but the
// poller stores one list, so a hybrid port is read like a trunk: its PVID is
// untagged and every other member shows as tagged.
export function vlanMembership(
  port: SwitchInterface,
  vid: number,
): VlanMembership {
  if (port.mode === "access") return port.pvid === vid ? "U" : "";
  if (port.mode !== "trunk" && port.mode !== "hybrid") return "";
  if (!parseVlanList(port.allowedVlans).has(vid)) return "";
  return port.pvid === vid ? "U" : "T";
}

/** Ports that switch VLANs, trunks included, in natural order. */
export function matrixPorts(ports: SwitchInterface[]): SwitchInterface[] {
  return ports
    .filter((p) => p.mode !== "")
    .sort((a, b) => a.name.localeCompare(b.name, "en", { numeric: true }));
}
