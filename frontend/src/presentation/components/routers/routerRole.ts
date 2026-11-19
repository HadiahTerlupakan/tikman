import type { MikrotikRouter, RouterRole } from "@/domain/entities";

export const ROLE_LABEL: Record<RouterRole, string> = {
  border: "Border",
  bras: "BRAS",
  other: "Lainnya",
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

/**
 * Tabs a router's role makes meaningful, in order; the first is where the
 * page opens. A BRAS has no upstream BGP to show, only its link to the border.
 */
export function tabKeysFor(role: RouterRole): string[] {
  return role === "bras" ? BRAS_TABS : BORDER_TABS;
}

export function isBras(router: Pick<MikrotikRouter, "role">): boolean {
  return router.role === "bras";
}
