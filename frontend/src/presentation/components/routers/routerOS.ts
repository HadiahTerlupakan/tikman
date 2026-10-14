/**
 * Building blocks for the RouterOS commands the drafts print. Every value in
 * a draft comes from the router or the database, but a draft is pasted into
 * a border router's terminal: a name holding a quote or a bracket must not be
 * able to end the string and add a command of its own.
 */

/** A RouterOS string literal: backslash, quote and $ (variables) escaped. */
export function quoted(text: string): string {
  return `"${text.replace(/[\\"$]/g, (c) => `\\${c}`)}"`;
}

const REGEX_SPECIAL = /[\\^$.|?*+()[\]{}]/g;

/**
 * A find expression for the running sessions of one connection: RouterOS
 * names them "<connection>-<n>", and a prefix match would also catch
 * "<connection>-NEW-1".
 */
export function sessionsOf(connection: string): string {
  const exact = connection.replace(REGEX_SPECIAL, (c) => `\\${c}`);
  return `[find where name~${quoted(`^${exact}-[0-9]+$`)}]`;
}

/**
 * A draft in the order it is meant to be run: read-only checks first, the
 * change, a check of its effect, and the undo. note is shown as text, never
 * pasted, so names in it need no escaping.
 */
export interface Draft {
  note: string;
  precheck: string[];
  apply: string[];
  check: string[];
  undo: string[];
}
