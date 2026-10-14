import { describe, expect, it } from "vitest";
import type { OwnPrefix, OwnPrefixPeer } from "@/domain/entities";
import { inboundDraft, inboundTargets, rewritePrepend } from "./ownPrefixDraft";

const peer = (over: Partial<OwnPrefixPeer>): OwnPrefixPeer => ({
  peer: "peer-cdn",
  transit: false,
  sessions: ["peer-cdn"],
  outputFilter: "CDN_OUT",
  rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }",
  announced: true,
  prepend: 2,
  downloadBytes: 0,
  uploadBytes: 0,
  ...over,
});

const own: OwnPrefix = {
  prefix: "203.0.113.0/24",
  downloadBytes: 1000,
  uploadBytes: 10,
  peers: [
    peer({}),
    peer({
      peer: "peer-ix-rs1",
      sessions: ["peer-ix-rs1", "peer-ix-rs2"],
      outputFilter: "IX_OUT",
      rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }",
      prepend: 0,
    }),
    peer({
      peer: "peer-transit",
      transit: true,
      sessions: ["peer-transit"],
      outputFilter: "TRANSIT_OUT",
      rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 6; accept; }",
      prepend: 6,
    }),
    peer({
      peer: "peer-other",
      outputFilter: "OTHER_OUT",
      rule: "",
      announced: false,
      prepend: 0,
    }),
  ],
};

describe("own prefix drafts", () => {
  it("sets, adds and removes a prepend in a rule's text", () => {
    const rule =
      "if (dst in 203.0.113.0/24) { set bgp-path-prepend 2; accept; }";
    expect(rewritePrepend(rule, 5)).toBe(
      "if (dst in 203.0.113.0/24) { set bgp-path-prepend 5; accept; }",
    );
    expect(rewritePrepend(rule, 0)).toBe(
      "if (dst in 203.0.113.0/24) { accept; }",
    );
    expect(rewritePrepend("if (dst in 203.0.113.0/24) { accept; }", 3)).toBe(
      "if (dst in 203.0.113.0/24) { set bgp-path-prepend 3; accept; }",
    );
  });

  it("only offers peers with a rule announcing the prefix", () => {
    expect(inboundTargets(own).map((p) => p.peer)).toEqual([
      "peer-cdn",
      "peer-ix-rs1",
      "peer-transit",
    ]);
  });

  it("clears the target's prepend and lengthens the others to the gap", () => {
    const d = inboundDraft(own, "peer-cdn");
    const draft = d.apply.join("\n");
    const everything = [...d.precheck, ...d.apply, ...d.check, ...d.undo].join(
      "\n",
    );

    expect(d.precheck).toContain(
      '/routing/filter/rule print where chain="CDN_OUT" and rule="if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }"',
    );
    expect(d.check).toContain(
      '/routing/filter/rule print where chain="CDN_OUT" and rule="if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }"',
    );
    expect(draft).toContain(
      '/routing/filter/rule set [find where chain="CDN_OUT" and rule="if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }"] rule="if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }"',
    );
    expect(draft).toContain(
      'rule="if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 3; accept; }"',
    );
    expect(everything).not.toContain('chain="TRANSIT_OUT"');
    expect(draft).toContain(
      '/routing/bgp/session resend [find where name~"^peer-ix-rs2-[0-9]+\\$"] afi=ip',
    );
    expect(d.undo.join("\n")).toContain(
      'rule="if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }"',
    );
  });

  it("says so when the prefix already leans towards the target", () => {
    const leaning: OwnPrefix = {
      ...own,
      peers: own.peers.map((p) =>
        p.peer === "peer-cdn"
          ? {
              ...p,
              prepend: 0,
              rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }",
            }
          : p.peer === "peer-ix-rs1"
            ? {
                ...p,
                prepend: 3,
                rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 3; accept; }",
              }
            : p,
      ),
    };

    const d = inboundDraft(leaning, "peer-cdn");
    expect(d.note).toContain("tidak ada yang perlu diubah");
    expect(d.apply).toEqual([]);
  });

  it("escapes RouterOS string specials in a rule", () => {
    const odd: OwnPrefix = {
      ...own,
      peers: [
        peer({
          rule: 'if (dst in 203.0.113.0/24) { set comment "$x"; set bgp-path-prepend 2; accept; }',
        }),
      ],
    };

    expect(inboundDraft(odd, "peer-cdn").apply.join("\n")).toContain(
      'rule="if (dst in 203.0.113.0/24) { set comment \\"\\$x\\"; accept; }"',
    );
  });

  it("keeps a hostile chain or session name from adding a command", () => {
    const hostile: OwnPrefix = {
      ...own,
      peers: [
        peer({
          outputFilter: 'OUT" ; /system reboot ; :put "',
          sessions: ['s"] ; /system reboot ; ["'],
        }),
        peer({
          peer: "peer-ix-rs1",
          outputFilter: "IX_OUT",
          prepend: 0,
          rule: "if (dst in 203.0.113.0/24) { accept; }",
        }),
      ],
    };

    const d = inboundDraft(hostile, "peer-ix-rs1");
    expect(
      [...d.precheck, ...d.apply, ...d.check, ...d.undo].join("\n"),
    ).not.toMatch(/[^\\]" ; \/system reboot/);
  });
});
