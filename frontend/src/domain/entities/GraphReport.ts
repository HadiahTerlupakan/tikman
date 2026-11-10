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

/** The body of a create or update; ports go in send order. */
export interface GraphReportInput {
  waGroupId: string;
  customerName: string;
  enabled: boolean;
  ports: GraphReportPortInput[];
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
