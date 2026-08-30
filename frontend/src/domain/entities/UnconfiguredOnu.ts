export interface UnconfiguredOnu {
  slot: number;
  port: number;
  serialNumber: string;
  deviceType?: string;
  softwareVersion?: string;
}

/**
 * An unconfigured ONU together with the OLT that detected it. The scan endpoint
 * is per-OLT and its rows carry no owner, so the page attaches one while merging
 * the scans — registration needs to know which OLT to send the commands to.
 */
export interface DetectedOnu extends UnconfiguredOnu {
  oltId: string;
  oltName: string;
}
