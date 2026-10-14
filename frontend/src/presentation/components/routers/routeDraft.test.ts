import { describe, expect, it } from "vitest";
import type { RouteAdvice, RoutePath } from "@/domain/entities";
import {
  draftTargets,
  preferDraft,
  rangeOptions,
  supernet,
} from "./routeDraft";

const active: RoutePath = {
  peer: "peer-cdn",
  remoteAs: 64511,
  transit: false,
  asPath: "64511,64520,64521,64496",
  pathLength: 4,
  localPref: 600,
  inputFilter: "CDN_IN",
};
const smallIX: RoutePath = {
  peer: "peer-small-ix",
  remoteAs: 64512,
  transit: false,
  asPath: "64512,64496",
  pathLength: 2,
  localPref: 300,
  inputFilter: "SMALLIX_IN",
};
const advice: RouteAdvice = {
  prefix: "198.51.100.0/24",
  asn: 64496,
  name: "EXAMPLE-NET",
  downloadBytes: 1000,
  uploadBytes: 10,
  status: "shorter_outbound",
  active,
  alternatives: [
    smallIX,
    { ...smallIX, peer: "peer-unknown", inputFilter: "" },
  ],
  inbound: null,
  suggested: "peer-small-ix",
  checkedAt: "2026-10-14T03:00:00Z",
};

describe("route drafts", () => {
  it("widens a prefix to a covering range", () => {
    expect(supernet("198.51.100.0/24", 22)).toBe("198.51.100.0/22");
    expect(supernet("198.51.103.0/24", 22)).toBe("198.51.100.0/22");
    expect(supernet("10.1.2.0/24", 8)).toBe("10.0.0.0/8");
  });

  it("offers the prefix's own length down to eight bits wider", () => {
    expect(rangeOptions("198.51.100.0/24")).toEqual([
      24, 23, 22, 21, 20, 19, 18, 17, 16,
    ]);
    expect(rangeOptions("10.0.0.0/12")).toEqual([12, 11, 10, 9, 8]);
  });

  it("only offers peers whose input chain is known", () => {
    expect(draftTargets(advice).map((p) => p.peer)).toEqual(["peer-small-ix"]);
  });

  it("drafts an exact-prefix rule in the target's chain, above every candidate", () => {
    const draft = preferDraft(advice, smallIX, "exact", 24);

    expect(draft).toContain('chain="SMALLIX_IN"');
    expect(draft).toContain(
      'rule="if (dst == 198.51.100.0/24) { set bgp-local-pref 610; accept; }"',
    );
    expect(draft).toContain(
      'place-before=[find where chain="SMALLIX_IN" and rule~"set bgp-local-pref" and !(comment~"^tikman")]',
    );
    expect(draft).toContain(
      '/routing/bgp/session refresh [find where name~"^peer-small-ix-[0-9]+\\$"]',
    );
    expect(draft).toContain(
      '/routing/filter/rule remove [find where comment="tikman: 198.51.100.0/24 via peer-small-ix"]',
    );
  });

  it("drafts a range rule that also covers more specific prefixes", () => {
    expect(preferDraft(advice, smallIX, "range", 22)).toContain(
      'rule="if (dst in 198.51.100.0/22) { set bgp-local-pref 610; accept; }"',
    );
  });

  // The origin-AS pattern was checked on the router: "64496$" matches only
  // paths ending in 64496, not 164496 or 64496 mid-path.
  it("drafts a per-AS rule with a pattern test first", () => {
    const draft = preferDraft(advice, smallIX, "asn", 24);

    expect(draft).toContain(
      'rule="if (bgp-as-path 64496\\$) { set bgp-local-pref 610; accept; }"',
    );
    expect(draft).toContain(
      '/routing/filter/test-as-path-regexp regexp="64496\\$" as-path="64511,64520,64521,64496"',
    );
    expect(draft).toContain('comment="tikman: AS64496 via peer-small-ix"');
  });

  it("keeps a hostile chain or peer name from adding a command", () => {
    const hostile = {
      ...smallIX,
      peer: 'x"] ; /system reboot ; ["',
      inputFilter: 'IN" ; /system reboot ; :put "',
    };

    const draft = preferDraft(advice, hostile, "exact", 24);

    expect(draft).not.toMatch(/[^\\]" ; \/system reboot/);
    expect(draft).toContain('chain="IN\\" ; /system reboot ; :put \\""');
  });
});
