import { OntStatus } from "@/domain/entities";

// One list, so the table and the filter cannot drift apart. The table used to
// shout the raw enum — "DYING_GASP" — which is a name from the database, not a
// word for an operator.
export const ONT_STATUSES: Array<{
  value: OntStatus;
  label: string;
  color: string;
}> = [
  { value: OntStatus.ONLINE, label: "Online", color: "success" },
  { value: OntStatus.OFFLINE, label: "Offline", color: "default" },
  { value: OntStatus.LOS, label: "LOS", color: "warning" },
  { value: OntStatus.DYING_GASP, label: "Dying gasp", color: "error" },
  { value: OntStatus.UNKNOWN, label: "Unknown", color: "default" },
];

export function ontStatusLabel(status?: OntStatus) {
  return ONT_STATUSES.find((s) => s.value === status)?.label ?? "Unknown";
}

export function ontStatusColor(status?: OntStatus) {
  return ONT_STATUSES.find((s) => s.value === status)?.color ?? "default";
}
