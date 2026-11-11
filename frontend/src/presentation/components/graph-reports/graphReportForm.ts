import dayjs, { type Dayjs } from "dayjs";
import {
  DEFAULT_GRAPH_REPORT_SEND_TIME,
  type GraphReport,
  type GraphReportInput,
  type GraphReportPreviewInput,
  type LibrenmsDevice,
  type LibrenmsPort,
  type WaAccount,
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
  /** Only narrows the group picker; the group row already names its
   * number, so it is not sent. */
  waAccountId?: string;
  waGroupId?: string;
  customerName?: string;
  message?: string;
  enabled: boolean;
  mentionAll: boolean;
  /** Only its hour and minute are read; the TimePicker needs a Dayjs. */
  sendTime: Dayjs;
  ports: PortFormValue[];
}

export interface SelectOption<T> {
  value: T;
  label: string;
  disabled?: boolean;
}

const WIB = "Asia/Jakarta";
const SEND_TIME_FORMAT = "HH:mm";

/** Reads the API's "HH:MM" without the customParseFormat plugin; the date
 * part is today's and never sent. */
export function parseSendTime(value: string): Dayjs {
  const [hour, minute] = value.split(":").map(Number);
  return dayjs().hour(hour).minute(minute).second(0).millisecond(0);
}

export function formatSendTime(value: Dayjs): string {
  return value.format(SEND_TIME_FORMAT);
}

export function emptyFormValues(): GraphReportFormValues {
  return {
    message: "",
    enabled: true,
    mentionAll: false,
    sendTime: parseSendTime(DEFAULT_GRAPH_REPORT_SEND_TIME),
    ports: [{}],
  };
}

export function reportToFormValues(report: GraphReport): GraphReportFormValues {
  return {
    waAccountId: report.waAccountId || undefined,
    waGroupId: report.waGroupId,
    customerName: report.customerName,
    message: report.message,
    enabled: report.enabled,
    mentionAll: report.mentionAll,
    sendTime: parseSendTime(report.sendTime || DEFAULT_GRAPH_REPORT_SEND_TIME),
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
  values: Omit<GraphReportFormValues, "enabled" | "mentionAll" | "sendTime">,
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
    mentionAll: values.mentionAll,
    sendTime: formatSendTime(values.sendTime),
  };
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

/** The phone number in a JID, without the ":NN" device suffix a linked
 * session's own JID carries. */
export function waNumber(jid?: string): string {
  return jid?.split("@")[0].split(":")[0] ?? "";
}

/** A number that is not connected stays pickable: its groups are still
 * known, and the report will send once it reconnects. */
export function accountOptions(accounts: WaAccount[]): SelectOption<string>[] {
  return accounts.map((account) => {
    const number = waNumber(account.jid);
    const name = number ? `${account.label} — +${number}` : account.label;
    const label =
      account.status === "connected" ? name : `${name} (tidak terhubung)`;
    return { value: account.id, label };
  });
}

/** The sending number of a report being edited, falling back to its
 * group's account for a report listed without one. */
export function initialAccountId(
  report: GraphReport,
  groups: WaGroup[],
): string | undefined {
  if (report.waAccountId) return report.waAccountId;
  return groups.find((g) => g.id === report.waGroupId)?.waAccountId;
}

export interface GroupOptionScope {
  /** The chosen number; nothing is listed until one is. */
  waAccountId?: string;
  /** The edited report's own group, which must stay selectable. */
  currentGroupId?: string;
}

/** The account label of every group JID that already has a report, so the
 * same WhatsApp group reached through a second number is not sent twice. It
 * is read from the reports, not from the loaded groups, which hold only the
 * chosen number's rows. */
function reportedJids(reports: GraphReport[]): Map<string, string> {
  return new Map(reports.map((r) => [r.waGroupJid, r.waAccountLabel]));
}

/** A group the number cannot post in (left, or admins-only without us as
 * admin) would fail every morning, and one already holding a report would be
 * refused as a duplicate. The same group through another number is shown
 * disabled rather than hidden, so the admin sees why it cannot be picked. */
export function groupOptions(
  groups: WaGroup[],
  reports: GraphReport[],
  { waAccountId, currentGroupId }: GroupOptionScope = {},
): SelectOption<string>[] {
  const others = reports.filter((r) => r.waGroupId !== currentGroupId);
  const takenRows = new Set(others.map((r) => r.waGroupId));
  const takenJids = reportedJids(others);
  return groups
    .filter(
      (g) =>
        g.waAccountId === waAccountId &&
        sendBlockedReason(g) === null &&
        !takenRows.has(g.id),
    )
    .map((g) => {
      const name = g.name || g.jid;
      const by = takenJids.get(g.jid);
      if (by === undefined) return { value: g.id, label: name };
      return {
        value: g.id,
        label: `${name} (sudah ada laporan lewat ${by})`,
        disabled: true,
      };
    });
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
