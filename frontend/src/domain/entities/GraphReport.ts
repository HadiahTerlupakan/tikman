/** One LibreNMS port whose 24-hour traffic graph a report sends. */
export interface GraphReportPort {
  id: string;
  /** LibreNMS's device_id, not a TikMan id. */
  deviceId: number;
  /** Shown only; LibreNMS is addressed by deviceId. */
  deviceName: string;
  ifName: string;
  label: string;
  position: number;
}

/** A daily traffic report: one registered WhatsApp group and its ports. */
export interface GraphReport {
  id: string;
  waGroupId: string;
  groupName: string;
  /** The WhatsApp group's JID, the same through every number in it. */
  waGroupJid: string;
  /** The number that sends it: the group row's own account. */
  waAccountId: string;
  /** The account's label, or its number when it has none. */
  waAccountLabel: string;
  customerName: string;
  /** The caption's text; "" sends DEFAULT_GRAPH_REPORT_MESSAGE. */
  message: string;
  enabled: boolean;
  /** Tags every member but our own number, without naming them in the
   * caption. */
  mentionAll: boolean;
  /** "HH:MM", 24-hour, WIB: when the daily send's window opens. */
  sendTime: string;
  /** WIB calendar date (YYYY-MM-DD) of the last scheduled send. A manual
   * send does not move it. */
  lastSentOn: string | null;
  lastAttemptAt: string | null;
  /** "" when the last attempt succeeded. */
  lastError: string;
  ports: GraphReportPort[];
}

export interface GraphReportPortInput {
  deviceId: number;
  deviceName: string;
  ifName: string;
  label: string;
}

/** What the composed image and its caption are built from; ports go in the
 * order they are stacked. */
export interface GraphReportPreviewInput {
  customerName: string;
  message: string;
  ports: GraphReportPortInput[];
}

/** The body of a create or update. */
export interface GraphReportInput extends GraphReportPreviewInput {
  waGroupId: string;
  enabled: boolean;
  mentionAll: boolean;
  sendTime: string;
}

export interface LibrenmsDevice {
  deviceId: number;
  hostname: string;
  sysName: string;
}

export interface LibrenmsPort {
  portId: number;
  ifName: string;
  ifAlias: string;
  operStatus: string;
}

/** The API refuses a report with more ports than this. */
export const GRAPH_REPORT_MAX_PORTS = 10;

/** One image of a previewed report, with the caption the group would get. */
export interface GraphReportPreviewImage {
  image: Blob;
  caption: string;
}

/** The caption the API sends when a report's message is blank, shown as
 * the message field's placeholder; kept identical to the backend's. */
export const DEFAULT_GRAPH_REPORT_MESSAGE =
  "Selamat pagi Rekan,\n\nBerikut kami lampirkan hasil monitoring trafik 24 jam terakhir untuk {pelanggan}.\n\nTerima kasih";

/** What the API stores when a report gives no send time. */
export const DEFAULT_GRAPH_REPORT_SEND_TIME = "06:00";

/** The API refuses a longer message. */
export const GRAPH_REPORT_MESSAGE_MAX = 1000;
