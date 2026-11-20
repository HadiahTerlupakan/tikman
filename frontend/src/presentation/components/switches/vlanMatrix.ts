import type { SwitchInterface, SwitchVlan } from "@/domain/entities";

const DEFAULT_VLAN = 1;
// `display vlan` names an unnamed VLAN "VLAN 0010"; showing it adds nothing.
const GENERATED_NAME = /^VLAN \d{4}$/;
// Operators wrap CE descriptions in quotes or ** banners to make them stand
// out in the CLI; in a table they are only noise.
const DECORATION = /^[\s"*]+|[\s"*]+$/g;

export function cleanLabel(text: string): string {
  return text.replace(DECORATION, "");
}

export function vlanLabel(vlan: SwitchVlan): string {
  return GENERATED_NAME.test(vlan.name) ? "" : cleanLabel(vlan.name);
}

/**
 * A port nobody has touched: down, no description, still an access port on
 * VLAN 1. A CE6850 has dozens of them and they would bury the real columns.
 */
export function isIdlePort(port: SwitchInterface): boolean {
  return (
    port.mode === "access" &&
    port.pvid === DEFAULT_VLAN &&
    port.operStatus !== "up" &&
    cleanLabel(port.alias) === ""
  );
}

export interface MatrixColumn {
  port: SwitchInterface;
  /** The port number within its group, e.g. "12" for 10GE1/0/12. */
  short: string;
}

export interface PortGroup {
  /** e.g. "10GE1/0/", or "Eth-Trunk" for a port without a slot path. */
  prefix: string;
  ports: MatrixColumn[];
}

const TRAILING_NUMBER = /^(.*?)(\d+)$/;

function splitName(name: string): [prefix: string, short: string] {
  const cut = name.lastIndexOf("/") + 1;
  if (cut > 0) return [name.slice(0, cut), name.slice(cut)];
  const m = TRAILING_NUMBER.exec(name);
  return m ? [m[1], m[2]] : ["", name];
}

/** Consecutive ports sharing a slot prefix, in the order given. */
export function portGroups(ports: SwitchInterface[]): PortGroup[] {
  const groups: PortGroup[] = [];
  for (const port of ports) {
    const [prefix, short] = splitName(port.name);
    const last = groups[groups.length - 1];
    if (last && last.prefix === prefix) last.ports.push({ port, short });
    else groups.push({ prefix, ports: [{ port, short }] });
  }
  return groups;
}
