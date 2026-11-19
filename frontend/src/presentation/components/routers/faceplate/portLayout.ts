import type { RouterInterface } from "../../../../domain/entities/MikrotikRouter";

export type JackKind = "rj" | "sfp" | "qsfp";

export interface PlacedJack {
  name: string;
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
};

const AUTO_KINDS: [JackKind, RegExp][] = [
  ["rj", /^ether/],
  ["sfp", /^(sfp|combo)/],
  ["qsfp", /^qsfp/],
];

function autoGroups(names: string[]): Group[] {
  const groups: Group[] = [];
  for (const [kind, re] of AUTO_KINDS) {
    // "qsfp" also matches the sfp pattern, so the sfp group must exclude it.
    const ports = names.filter(
      (n) => re.test(n) && !(kind === "sfp" && /^qsfp/.test(n)),
    );
    if (ports.length === 0) continue;
    if (kind === "rj" && ports.length === 1) {
      groups.push({ kind, rows: 1, ports, mgmt: true });
    } else {
      groups.push({ kind, rows: ports.length > 1 ? 2 : 1, ports });
    }
  }
  return groups;
}

function placeGroup(group: Group, startX: number): PlacedJack[] {
  const geo = GEOMETRY[group.kind];
  return group.ports.map((name, i) => {
    const double = group.rows === 2;
    const col = double ? Math.floor(i / 2) : i;
    return {
      name,
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

/** Positions every jack of a router's front panel; a known board keeps its own template. */
export function layoutFaceplate(
  boardName: string | null,
  portNames: string[],
): FaceplateLayout {
  const template = boardName ? TEMPLATES[boardName] : undefined;
  const groups = template ?? autoGroups(portNames);
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
