import type { WaGroupPoll } from "@/domain/entities";

export interface PollOptionTally {
  name: string;
  count: number;
  /** Share of everyone who voted, 0 to 1. */
  share: number;
  voterNames: string[];
  mine: boolean;
}

/**
 * Counts per option, measured against the people who voted rather than the
 * votes cast: in a multiple-choice poll one person can back every option, and
 * WhatsApp's bars then all read full, not a third each.
 */
export function pollTally(poll: WaGroupPoll): {
  options: PollOptionTally[];
  voters: number;
} {
  const voters = new Set(
    poll.options.flatMap((o) => o.voters.map((v) => v.jid)),
  ).size;
  const options = poll.options.map((o) => ({
    name: o.name,
    count: o.voters.length,
    share: voters ? o.voters.length / voters : 0,
    voterNames: o.voters.map((v) => (v.isSelf ? "Anda" : v.name)),
    mine: o.voters.some((v) => v.isSelf),
  }));
  return { options, voters };
}

/**
 * The full set of options to send when one is clicked, or null when the click
 * would go past the poll's limit. A vote is always the whole selection: the
 * server replaces the previous one with it.
 */
export function nextVote(poll: WaGroupPoll, option: string): string[] | null {
  const mine = poll.options
    .filter((o) => o.voters.some((v) => v.isSelf))
    .map((o) => o.name);
  if (mine.includes(option)) return mine.filter((name) => name !== option);
  if (poll.selectableCount === 1) return [option];
  if (poll.selectableCount > 1 && mine.length >= poll.selectableCount) {
    return null;
  }
  return [...mine, option];
}
