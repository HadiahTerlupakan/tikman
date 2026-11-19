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
const BRAS_TABS = ["panel", "interfaces", "router"];

/** Tabs a router's role makes meaningful; a BRAS has no upstream BGP to show. */
export function tabKeysFor(role: RouterRole): string[] {
  return role === "bras" ? BRAS_TABS : BORDER_TABS;
}

export function isBras(router: Pick<MikrotikRouter, "role">): boolean {
  return router.role === "bras";
}
