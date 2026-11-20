import type { RouterInterface } from "../../../../domain/entities/MikrotikRouter";

export type JackKind = "rj" | "sfp" | "qsfp";

export interface PlacedJack {
  /** The interface's own name, which may be one the operator gave it. */
  name: string;
  /** The port as printed on the panel, e.g. "ether1" for "ether1-x86". */
  slot: string;
  kind: JackKind;
  row: "top" | "bottom";
  x: number;
  y: number;
  w: number;
  h: number;
  mgmt: boolean;
}

export interface FaceplateLayout {
  templated: boolean;
  model: string;
  extras: boolean;
  jacks: PlacedJack[];
  width: number;
}

export const CHASSIS_HEIGHT = 132;

interface Group {
  kind: JackKind;
  rows: 1 | 2;
  ports: string[];
  /** Panel names, parallel to ports, where a port was renamed. */
  slots?: string[];
  mgmt?: boolean;
}

interface Geometry {
  w: number;
  h: number;
  pitch: number;
  y2: [number, number];
  y1: number;
}

const GEOMETRY: Record<JackKind, Geometry> = {
  rj: { w: 46, h: 40, pitch: 58, y2: [22, 70], y1: 46 },
  sfp: { w: 58, h: 30, pitch: 72, y2: [30, 72], y1: 51 },
  qsfp: { w: 78, h: 34, pitch: 94, y2: [26, 72], y1: 49 },
};

const GROUP_GAP = 44;
const START_WITH_EXTRAS = 166;
const START_WITHOUT_EXTRAS = 40;
const TAIL_TEMPLATED = 230;
const TAIL_AUTOMATIC = 330;

const range = (prefix: string, from: number, to: number) =>
  Array.from({ length: to - from + 1 }, (_, i) => `${prefix}${from + i}`);

// Known boards mirror the real faceplate; any other board is grouped by name.
const TEMPLATES: Record<string, Group[]> = {
  "CCR2116-12G-4S+": [
    { kind: "rj", rows: 1, ports: ["ether1"], mgmt: true },
    { kind: "rj", rows: 2, ports: range("ether", 2, 13) },
    { kind: "sfp", rows: 2, ports: range("sfp-sfpplus", 1, 4) },
  ],
  "CCR2004-16G-2S+": [
    { kind: "rj", rows: 2, ports: range("ether", 1, 16) },
    { kind: "sfp", rows: 2, ports: range("sfp-sfpplus", 1, 2) },
  ],
  // Odd ports sit in the top row, as on the panel; the console left of
  // MEth0/0/0 is drawn by the chassis as the template extras.
  "CE6850-48S4Q-EI": [
    { kind: "rj", rows: 1, ports: ["MEth0/0/0"], mgmt: true },
    { kind: "sfp", rows: 2, ports: range("10GE1/0/", 1, 48) },
    { kind: "qsfp", rows: 2, ports: range("40GE1/0/", 1, 4) },
  ],
};

/** Which jack an interface name is, in the order the panel groups them. */
export type PortKinds = [JackKind, RegExp][];

export const ROUTEROS_KINDS: PortKinds = [
  ["rj", /^ether/],
  ["sfp", /^(sfp|combo)/],
  ["qsfp", /^qsfp/],
];

// MEth comes first and alone, so the automatic layout marks it as management.
export const HUAWEI_CE_KINDS: PortKinds = [
  ["rj", /^MEth\d+\/\d+\/\d+$/],
  ["rj", /^GE\d+\/\d+\/\d+$/],
  ["sfp", /^(10|25)GE\d+\/\d+\/\d+$/],
  ["qsfp", /^(40|100)GE\d+\/\d+\/\d+$/],
];

export const FIBERHOME_KINDS: PortKinds = [
  ["rj", /^gigaethernet/],
  ["sfp", /^xgigaethernet/],
  ["qsfp", /^40gigaethernet/],
];

/** The one rj port of a board is its management port, unless it was left over from a template. */
function autoGroups(
  names: string[],
  singleRjIsMgmt: boolean,
  kinds: PortKinds,
): Group[] {
  const groups: Group[] = [];
  for (const [kind, re] of kinds) {
    const ports = names.filter((n) => re.test(n));
    if (ports.length === 0) continue;
    const mgmt = singleRjIsMgmt && kind === "rj" && ports.length === 1;
    groups.push({ kind, rows: ports.length > 1 ? 2 : 1, ports, mgmt });
  }
  // A port renamed away from every pattern is still a port, and must still be drawn.
  const rest = names.filter((n) => !kinds.some(([, re]) => re.test(n)));
  if (rest.length > 0) {
    groups.push({ kind: "rj", rows: rest.length > 1 ? 2 : 1, ports: rest });
  }
  return groups;
}

/**
 * The interface sitting in a template slot: the slot's own name, or one an
 * operator extended it into, as RouterOS operators do ("ether1-x86",
 * "sfp-sfpplus1-AGREGATOR"). "ether12-wifi" must not land on ether1, so the
 * extension may not start with a digit.
 */
function occupant(slot: string, names: string[], taken: Set<string>): string {
  const free = names.filter((n) => !taken.has(n));
  return (
    free.find((n) => n === slot) ??
    free.find((n) => n.startsWith(slot) && /^\D/.test(n.slice(slot.length))) ??
    slot
  );
}

/** A template keeps every port in its place; names that fit no slot trail it, grouped by name. */
function templateGroups(
  template: Group[],
  names: string[],
  kinds: PortKinds,
): Group[] {
  const taken = new Set<string>();
  const placed = template.map((group) => {
    const ports = group.ports.map((slot) => {
      const name = occupant(slot, names, taken);
      taken.add(name);
      return name;
    });
    return { ...group, ports, slots: group.ports };
  });
  const rest = names.filter((n) => !taken.has(n));
  return [...placed, ...autoGroups(rest, false, kinds)];
}

function placeGroup(group: Group, startX: number): PlacedJack[] {
  const geo = GEOMETRY[group.kind];
  return group.ports.map((name, i) => {
    const double = group.rows === 2;
    const col = double ? Math.floor(i / 2) : i;
    return {
      name,
      slot: group.slots?.[i] ?? name,
      kind: group.kind,
      row: double && i % 2 === 0 ? "top" : "bottom",
      x: startX + col * geo.pitch,
      y: double ? geo.y2[i % 2] : geo.y1,
      w: geo.w,
      h: geo.h,
      mgmt: group.mgmt === true,
    };
  });
}

function groupWidth(group: Group): number {
  const geo = GEOMETRY[group.kind];
  const cols =
    group.rows === 2 ? Math.ceil(group.ports.length / 2) : group.ports.length;
  return (cols - 1) * geo.pitch + geo.w + GROUP_GAP;
}

/** Positions every jack of a device's front panel; a known board keeps its own template. */
export function layoutFaceplate(
  boardName: string | null,
  portNames: string[],
  kinds: PortKinds = ROUTEROS_KINDS,
): FaceplateLayout {
  const template = boardName ? TEMPLATES[boardName] : undefined;
  const groups = template
    ? templateGroups(template, portNames, kinds)
    : autoGroups(portNames, true, kinds);
  const extras = template !== undefined;
  let x = extras ? START_WITH_EXTRAS : START_WITHOUT_EXTRAS;
  const jacks: PlacedJack[] = [];
  for (const group of groups) {
    jacks.push(...placeGroup(group, x));
    x += groupWidth(group);
  }
  return {
    templated: extras,
    model: boardName ?? "",
    extras,
    jacks,
    width: x + (extras ? TAIL_TEMPLATED : TAIL_AUTOMATIC),
  };
}

/** Physical ports only: VLANs carry a parent and bridges or tunnels are not ether type. */
export function physicalPortNames(interfaces: RouterInterface[]): string[] {
  return interfaces
    .filter((i) => i.type === "ether" && !i.parentName)
    .map((i) => i.name)
    .sort((a, b) => a.localeCompare(b, "en", { numeric: true }));
}
