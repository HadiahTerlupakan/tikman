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
import { GENERAL_TOPIC, telegramDestinationLabel } from "./telegramDestination";

/** One port row as the form holds it; blank until the operator picks. */
export interface PortFormValue {
  deviceId?: number;
  deviceName?: string;
  ifName?: string;
  label?: string;
  notifyStatus?: boolean;
}

/** Where the report goes; the destinations of an unchosen channel are
 * kept in the form but never sent. */
export type DeliveryChannel = "whatsapp" | "telegram" | "both";

export const CHANNEL_OPTIONS: SelectOption<DeliveryChannel>[] = [
  { value: "whatsapp", label: "WhatsApp saja" },
  { value: "telegram", label: "Telegram saja" },
  { value: "both", label: "WhatsApp & Telegram" },
];

export function sendsWhatsApp(channel?: DeliveryChannel): boolean {
  return channel === "whatsapp" || channel === "both";
}

export function sendsTelegram(channel?: DeliveryChannel): boolean {
  return channel === "telegram" || channel === "both";
}

/** The channel saved destinations imply; none at all opens on WhatsApp. */
export function channelOf(
  waGroupId: string | null,
  telegramChatId: string | null,
): DeliveryChannel {
  if (waGroupId && telegramChatId) return "both";
  return telegramChatId ? "telegram" : "whatsapp";
}

export interface GraphReportFormValues {
  channel: DeliveryChannel;
  /** Only narrows the group picker; the group row already names its
   * number, so it is not sent. */
  waAccountId?: string;
  waGroupId?: string;
  telegramChatId?: string;
  /** GENERAL_TOPIC for General; only read while the chat is a forum. */
  telegramThreadId?: number;
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
    channel: "whatsapp",
    message: "",
    enabled: true,
    mentionAll: false,
    sendTime: parseSendTime(DEFAULT_GRAPH_REPORT_SEND_TIME),
    ports: [{}],
  };
}

export function reportToFormValues(report: GraphReport): GraphReportFormValues {
  return {
    channel: channelOf(report.waGroupId, report.telegramChatId),
    waAccountId: report.waAccountId || undefined,
    waGroupId: report.waGroupId ?? undefined,
    telegramChatId: report.telegramChatId ?? undefined,
    telegramThreadId: report.telegramThreadId ?? GENERAL_TOPIC,
    customerName: report.customerName,
    message: report.message,
    enabled: report.enabled,
    mentionAll: report.mentionAll,
    sendTime: parseSendTime(report.sendTime || DEFAULT_GRAPH_REPORT_SEND_TIME),
    ports: [...report.ports]
      .sort((a, b) => a.position - b.position)
      .map(({ deviceId, deviceName, ifName, label, notifyStatus }) => ({
        deviceId,
        deviceName,
        ifName,
        label,
        notifyStatus,
      })),
  };
}

/** The form's rules have already required every field; the API stacks the
 * ports' graphs in this order. A preview needs no group, so it is built
 * without one. */
export function toPreviewInput(
  values: Omit<
    GraphReportFormValues,
    "channel" | "enabled" | "mentionAll" | "sendTime"
  >,
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
  // A cleared Select leaves undefined; null tells the API "none" on an
  // update as well as a create. Switching a channel off keeps its picks in
  // the form, so switching back does not lose them, but they are not sent.
  const waGroupId = sendsWhatsApp(values.channel)
    ? values.waGroupId ?? null
    : null;
  const telegramChatId = sendsTelegram(values.channel)
    ? values.telegramChatId ?? null
    : null;
  const preview = toPreviewInput(values);
  return {
    ...preview,
    ports: preview.ports.map((port, i) => ({
      ...port,
      notifyStatus: values.ports[i].notifyStatus ?? false,
    })),
    waGroupId,
    telegramChatId,
    // Changing the chat resets the topic, so a thread id only survives
    // while it belongs to the chosen forum.
    telegramThreadId:
      telegramChatId && values.telegramThreadId
        ? values.telegramThreadId
        : null,
    enabled: values.enabled,
    // Switching number clears the group but leaves the box ticked, so a
    // later pick on the new number keeps the choice; with no group there is
    // no one to tag.
    mentionAll: Boolean(waGroupId) && values.mentionAll,
    sendTime: formatSendTime(values.sendTime),
  };
}

/** The report's destinations by name, for the send confirmation. */
export function destinationNames(report: GraphReport): string {
  return [
    report.waGroupId && report.groupName,
    report.telegramChatId && telegramDestinationLabel(report),
  ]
    .filter(Boolean)
    .join(" dan ");
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
  return new Map(
    reports
      .filter((r) => r.waGroupId)
      .map((r) => [r.waGroupJid, r.waAccountLabel]),
  );
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
