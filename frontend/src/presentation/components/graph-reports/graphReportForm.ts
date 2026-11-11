import {
  DEFAULT_GRAPH_REPORT_MESSAGE,
  GRAPH_REPORT_CUSTOMER_PLACEHOLDER,
  type GraphReport,
  type GraphReportInput,
  type GraphReportPreviewInput,
  type LibrenmsDevice,
  type LibrenmsPort,
  type WaGroup,
} from "@/domain/entities";
import { sendBlockedReason } from "../cs/groups/groupSummary";

/** One port row as the form holds it; blank until the operator picks. */
export interface PortFormValue {
  deviceId?: number;
  deviceName?: string;
  ifName?: string;
  label?: string;
}

export interface GraphReportFormValues {
  waGroupId?: string;
  customerName?: string;
  message?: string;
  enabled: boolean;
  ports: PortFormValue[];
}

export interface SelectOption<T> {
  value: T;
  label: string;
}

const WIB = "Asia/Jakarta";

export function emptyFormValues(): GraphReportFormValues {
  return { message: "", enabled: true, ports: [{}] };
}

export function reportToFormValues(report: GraphReport): GraphReportFormValues {
  return {
    waGroupId: report.waGroupId,
    customerName: report.customerName,
    message: report.message,
    enabled: report.enabled,
    ports: [...report.ports]
      .sort((a, b) => a.position - b.position)
      .map(({ deviceId, deviceName, ifName, label }) => ({
        deviceId,
        deviceName,
        ifName,
        label,
      })),
  };
}

/** The form's rules have already required every field; the API stacks the
 * ports' graphs in this order. A preview needs no group, so it is built
 * without one. */
export function toPreviewInput(
  values: Omit<GraphReportFormValues, "enabled">,
): GraphReportPreviewInput {
  return {
    customerName: (values.customerName ?? "").trim(),
    message: (values.message ?? "").trim(),
    ports: (values.ports ?? []).map((port) => ({
      deviceId: port.deviceId as number,
      deviceName: port.deviceName ?? "",
      ifName: port.ifName ?? "",
      label: (port.label ?? "").trim(),
    })),
  };
}

export function toGraphReportInput(
  values: GraphReportFormValues,
): GraphReportInput {
  return {
    ...toPreviewInput(values),
    waGroupId: values.waGroupId ?? "",
    enabled: values.enabled,
  };
}

/** The caption the API writes under the image, so the preview can show it
 * before anything is sent. */
export function reportCaption(message: string, customerName: string): string {
  const text = message.trim() || DEFAULT_GRAPH_REPORT_MESSAGE;
  return text.split(GRAPH_REPORT_CUSTOMER_PLACEHOLDER).join(customerName);
}

/** The API's update replaces the whole report, so flipping "Aktif" resends
 * everything else unchanged. */
export function reportWithEnabled(
  report: GraphReport,
  enabled: boolean,
): GraphReportInput {
  return toGraphReportInput({ ...reportToFormValues(report), enabled });
}

/** An interface description names the customer far more often than
 * "ether5" does. */
export function defaultPortLabel(port: LibrenmsPort): string {
  return port.ifAlias.trim() || port.ifName;
}

export function portOptionLabel(port: LibrenmsPort): string {
  const alias = port.ifAlias.trim();
  return alias ? `${port.ifName} — ${alias}` : port.ifName;
}

export function deviceDisplayName(device: LibrenmsDevice): string {
  return device.sysName || device.hostname;
}

export function deviceOptions(
  devices: LibrenmsDevice[],
  current?: PortFormValue,
): SelectOption<number>[] {
  const options = devices.map((device) => ({
    value: device.deviceId,
    label:
      device.sysName && device.sysName !== device.hostname
        ? `${device.sysName} (${device.hostname})`
        : device.hostname,
  }));
  // A saved port whose device LibreNMS no longer lists, or has not answered
  // for yet, would otherwise show as a bare device_id.
  const id = current?.deviceId;
  if (id !== undefined && !options.some((o) => o.value === id)) {
    options.push({ value: id, label: current?.deviceName || String(id) });
  }
  return options;
}

/** A group the number cannot post in (left, or admins-only without us as
 * admin) would fail every morning, and one already holding a report would be
 * refused as a duplicate. */
export function groupOptions(
  groups: WaGroup[],
  reports: GraphReport[],
  currentGroupId?: string,
): SelectOption<string>[] {
  const taken = new Set(
    reports.map((r) => r.waGroupId).filter((id) => id !== currentGroupId),
  );
  return groups
    .filter((g) => sendBlockedReason(g) === null && !taken.has(g.id))
    .map((g) => ({ value: g.id, label: g.name || g.jid }));
}

/** lastSentOn is a bare WIB date, so it is formatted as UTC to stop the
 * browser's own zone from shifting it a day. */
export function formatSentOn(date: string | null): string {
  if (!date) return "Belum pernah";
  return new Date(`${date}T00:00:00Z`).toLocaleDateString("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function formatAttemptAt(timestamp: string | null): string {
  if (!timestamp) return "";
  const text = new Date(timestamp).toLocaleString("id-ID", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: WIB,
  });
  return `${text} WIB`;
}
