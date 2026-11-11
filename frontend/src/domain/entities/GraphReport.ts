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
  customerName: string;
  /** The caption's text; "" sends DEFAULT_GRAPH_REPORT_MESSAGE. */
  message: string;
  enabled: boolean;
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

/** Replaced in the caption by the report's customer name. */
export const GRAPH_REPORT_CUSTOMER_PLACEHOLDER = "{pelanggan}";

/** The caption the API sends when a report's message is blank; kept
 * identical to the backend's so the preview shows what the group gets. */
export const DEFAULT_GRAPH_REPORT_MESSAGE =
  "Selamat pagi Rekan,\n\nBerikut kami lampirkan hasil monitoring trafik 24 jam terakhir untuk {pelanggan}.\n\nTerima kasih";

/** The API refuses a longer message. */
export const GRAPH_REPORT_MESSAGE_MAX = 1000;
