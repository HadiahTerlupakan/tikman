import type { RouteAdvice, RoutePath } from "@/domain/entities";

export type DraftScope = "exact" | "range" | "asn";

/** How far above every candidate's local-pref the drafted rule goes: enough to win. */
const LOCAL_PREF_STEP = 10;
/** A range is offered up to this many bits shorter than the prefix. */
const MAX_RANGE_WIDENING = 8;
const SHORTEST_RANGE = 8;
const IPV4_BITS = 32;
const OCTET_BITS = 8;
const OCTETS = 4;
const OCTET_MASK = 0xff;

function parsePrefix(prefix: string): { addr: number; bits: number } {
  const [ip, bits] = prefix.split("/");
  const addr = ip
    .split(".")
    .reduce((acc, octet) => acc * 256 + Number(octet), 0);
  return { addr, bits: Number(bits) };
}

/** The prefix widened to bits, e.g. 198.51.100.0/24 at 22 is 198.51.100.0/22. */
export function supernet(prefix: string, bits: number): string {
  const { addr } = parsePrefix(prefix);
  const size = 2 ** (IPV4_BITS - bits);
  const network = Math.floor(addr / size) * size;
  const octets = Array.from(
    { length: OCTETS },
    (_, i) =>
      Math.floor(network / 2 ** (OCTET_BITS * (OCTETS - 1 - i))) & OCTET_MASK,
  );
  return `${octets.join(".")}/${bits}`;
}

/** Range lengths to offer: the prefix's own, then wider, down to /8. */
export function rangeOptions(prefix: string): number[] {
  const { bits } = parsePrefix(prefix);
  const out: number[] = [];
  for (
    let b = bits;
    b >= Math.max(SHORTEST_RANGE, bits - MAX_RANGE_WIDENING);
    b--
  ) {
    out.push(b);
  }
  return out;
}

/** The peers a rule could prefer: every candidate but the active one. */
export function draftTargets(advice: RouteAdvice): RoutePath[] {
  return (advice.alternatives ?? []).filter((p) => p.inputFilter !== "");
}

function matcher(
  advice: RouteAdvice,
  scope: DraftScope,
  rangeBits: number,
): { condition: string; label: string } {
  switch (scope) {
    case "asn":
      return {
        condition: `bgp-as-path ${advice.asn}$`,
        label: `AS${advice.asn}`,
      };
    case "range": {
      const range = supernet(advice.prefix, rangeBits);
      return { condition: `dst in ${range}`, label: range };
    }
    default:
      return { condition: `dst == ${advice.prefix}`, label: advice.prefix };
  }
}

/**
 * RouterOS v7 commands, for the operator to review and run by hand, that make
 * outbound traffic for the chosen scope prefer target: a rule in target's
 * input chain, before that chain's own local-pref rule so its bogon filters
 * still apply, then a route refresh, a check and the undo.
 */
export function preferDraft(
  advice: RouteAdvice,
  target: RoutePath,
  scope: DraftScope,
  rangeBits: number,
): string {
  const chain = target.inputFilter;
  const { condition, label } = matcher(advice, scope, rangeBits);
  const candidates = (advice.alternatives ?? []).concat(
    advice.active ? [advice.active] : [],
  );
  const pref =
    Math.max(...candidates.map((p) => p.localPref)) + LOCAL_PREF_STEP;
  const comment = `tikman: ${label} via ${target.peer}`;
  const refresh = `/routing/bgp/session refresh [find where name~"^${target.peer}"]`;
  const lines = [
    `# Keluar ke ${label} lewat ${target.peer}: hanya trafik keluar (upload)`,
  ];
  if (scope === "asn") {
    lines.push(
      `# Berlaku untuk semua prefix AS${advice.asn} yang diterima dari ${target.peer}. Uji polanya dulu:`,
      `/routing/filter/test-as-path-regexp regexp="${advice.asn}$" as-path="${(advice.active?.asPath ?? "").trim()}"`,
    );
  }
  lines.push(
    `/routing/filter/rule add chain=${chain} comment="${comment}" \\`,
    `  place-before=[find where chain="${chain}" and rule~"set bgp-local-pref" and !(comment~"^tikman")] \\`,
    `  rule="if (${condition}) { set bgp-local-pref ${pref}; accept; }"`,
    refresh,
    "# Cek: rute aktif harus pindah ke peer di atas",
    `/routing/route print where dst-address=${advice.prefix}`,
    "# Batalkan",
    `/routing/filter/rule remove [find where comment="${comment}"]`,
    refresh,
  );
  return lines.join("\n");
}
