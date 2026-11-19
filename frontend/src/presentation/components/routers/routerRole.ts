import type { MikrotikRouter, RouterRole } from "@/domain/entities";

export const ROLE_LABEL: Record<RouterRole, string> = {
  border: "Border",
  bras: "BRAS",
  other: "Lainnya",
};

/** A role as its label-maker tape spells it. */
export const ROLE_TAPE: Record<RouterRole, string> = {
  border: "BORDER",
  bras: "BRAS",
  other: "LAINNYA",
};

const BORDER_TABS = [
  "panel",
  "upstream",
  "paths",
  "flows",
  "interfaces",
  "router",
];
const BRAS_TABS = ["ringkasan", "panel", "trafik", "interfaces", "router"];
const OTHER_TABS = ["panel", "interfaces", "router"];

/**
 * Tabs a router's role makes meaningful, in order; the first is where the
 * page opens. A BRAS has no upstream BGP to show, only its link to the border,
 * and a router of another role is shown only as a box of ports.
 */
export function tabKeysFor(role: RouterRole): string[] {
  switch (role) {
    case "bras":
      return BRAS_TABS;
    case "other":
      return OTHER_TABS;
    default:
      return BORDER_TABS;
  }
}

export function isBras(router: Pick<MikrotikRouter, "role">): boolean {
  return router.role === "bras";
}
