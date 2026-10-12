import type { MikrotikRouter } from "@/domain/entities";

/** A complete router for component tests; override what the test is about. */
export function mikrotikRouter(
  over: Partial<MikrotikRouter> = {},
): MikrotikRouter {
  return {
    id: "r1",
    name: "Border",
    host: "192.0.2.1",
    siteId: null,
    snmpPort: 161,
    apiUsername: "tikman",
    apiPort: 8443,
    apiProtocol: "rest",
    enabled: true,
    tlsFingerprint: "ab".repeat(32),
    presentedFingerprint: null,
    status: "up",
    apiStatus: "ok",
    apiError: null,
    cpuLoad: null,
    memoryTotal: null,
    memoryFree: null,
    uptimeSeconds: null,
    rosVersion: null,
    boardName: null,
    lastSeenAt: null,
    flowEnabled: false,
    flowSampling: 100,
    flowLastSeenAt: null,
    bgpEstablished: 0,
    bgpTotal: 0,
    ...over,
  };
}
