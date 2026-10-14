import type { OwnPrefix, OwnPrefixPeer } from "@/domain/entities";
import { type Draft, quoted, sessionsOf } from "./routerOS";

/** How many AS hops longer the other peers' paths are made than the target's. */
export const INBOUND_PREPEND_GAP = 3;

const PREPEND = /set bgp-path-prepend \d+;\s*/;

/** The rule with its prepend set to n: removed at 0, added before accept. */
export function rewritePrepend(rule: string, n: number): string {
  if (PREPEND.test(rule)) {
    return rule.replace(PREPEND, n === 0 ? "" : `set bgp-path-prepend ${n}; `);
  }
  if (n === 0) return rule;
  return rule.replace("accept;", `set bgp-path-prepend ${n}; accept;`);
}

function setRule(peer: OwnPrefixPeer, from: string, to: string): string {
  return `/routing/filter/rule set [find where chain=${quoted(peer.outputFilter)} and rule=${quoted(from)}] rule=${quoted(to)}`;
}

function resend(peers: OwnPrefixPeer[]): string[] {
  return peers.flatMap((p) =>
    p.sessions.map((s) => `/routing/bgp/session resend ${sessionsOf(s)}`),
  );
}

/** The peers whose announcement of the prefix a draft can change. */
export function inboundTargets(own: OwnPrefix): OwnPrefixPeer[] {
  return own.peers.filter((p) => p.announced && p.rule !== "");
}

/**
 * RouterOS v7 commands, for the operator to review and run by hand, that
 * make download for the prefix lean towards target: no prepend there, at
 * least INBOUND_PREPEND_GAP on every other peer announcing it, then a resend.
 * The precheck lists, read-only, each rule the change will rewrite; the undo
 * restores each rule's exact text.
 */
export function inboundDraft(own: OwnPrefix, target: string): Draft {
  const changes = inboundTargets(own)
    .map((peer) => {
      const prepend =
        peer.peer === target ? 0 : Math.max(peer.prepend, INBOUND_PREPEND_GAP);
      return { peer, next: rewritePrepend(peer.rule, prepend) };
    })
    .filter((c) => c.next !== c.peer.rule);
  if (changes.length === 0) {
    return {
      note: `${own.prefix} sudah diumumkan paling pendek lewat ${target}; tidak ada yang perlu diubah.`,
      precheck: [],
      apply: [],
      check: [],
      undo: [],
    };
  }
  const touched = changes.map((c) => c.peer);
  const find = (peer: OwnPrefixPeer, rule: string) =>
    `/routing/filter/rule print where chain=${quoted(peer.outputFilter)} and rule=${quoted(rule)}`;
  return {
    note: `Download untuk ${own.prefix} cenderung masuk lewat ${target}. Prepend hanya membuat jalur lain terlihat lebih panjang; jaringan pengirim tetap yang memutuskan, jadi lihat hasilnya di panel ini satu sampai dua jam kemudian.`,
    precheck: changes.map((c) => find(c.peer, c.peer.rule)),
    apply: [
      ...changes.map((c) => setRule(c.peer, c.peer.rule, c.next)),
      ...resend(touched),
    ],
    check: changes.map((c) => find(c.peer, c.next)),
    undo: [
      ...changes.map((c) => setRule(c.peer, c.next, c.peer.rule)),
      ...resend(touched),
    ],
  };
}
