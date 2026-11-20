import type {
  ManagedSwitch,
  SwitchInterface,
  SwitchNeighbor,
  SwitchVlan,
} from "@/domain/entities";

export const fixtureSwitch = (
  over: Partial<ManagedSwitch> = {},
): ManagedSwitch =>
  ({
    id: "s-1",
    name: "SW-TEST",
    host: "192.0.2.10",
    vendor: "huawei_ce",
    model: "CE6850-48S4Q-EI",
    version: "V100R006C00",
    protocol: "ssh",
    enabled: true,
    status: "up",
    hostKeyChanged: false,
    hostKeyFingerprint: "",
    presentedFingerprint: null,
    rackPosition: null,
    ...over,
  }) as ManagedSwitch;

export const fixturePort = (
  over: Partial<SwitchInterface> & { name: string },
): SwitchInterface =>
  ({
    id: over.name,
    switchId: "s-1",
    alias: "",
    kind: "sfp",
    ifIndex: 1,
    adminUp: true,
    operStatus: "up",
    speedMbps: 10000,
    mode: "",
    pvid: null,
    allowedVlans: "",
    stpEdge: null,
    lastInBps: 1e6,
    lastOutBps: 1e5,
    inErrors: 0,
    outErrors: 0,
    inDiscards: 0,
    lastPolledAt: null,
    ...over,
  }) as SwitchInterface;

export const fixturePorts: SwitchInterface[] = [
  fixturePort({
    name: "10GE1/0/1",
    alias: "ke-olt",
    mode: "trunk",
    pvid: 1,
    allowedVlans: "10,20-22",
  }),
  fixturePort({
    name: "10GE1/0/2",
    mode: "access",
    pvid: 30,
    operStatus: "down",
    stpEdge: true,
  }),
  fixturePort({
    name: "10GE1/0/3",
    mode: "hybrid",
    pvid: 10,
    allowedVlans: "10,30",
  }),
  fixturePort({ name: "Vlanif10", kind: "virtual" }),
];

export const fixtureVlans: SwitchVlan[] = [
  { switchId: "s-1", vid: 10, name: "mgmt" },
  { switchId: "s-1", vid: 20, name: "pelanggan" },
  { switchId: "s-1", vid: 30, name: "olt" },
];

export const fixtureNeighbors: SwitchNeighbor[] = [
  {
    id: "n1",
    switchId: "s-1",
    port: "10GE1/0/1",
    kind: "lldp",
    mac: "",
    vid: null,
    remoteName: "OLT-TEST",
    remotePort: "gei-1/1",
    remotePlatform: "C300",
  },
  {
    id: "n2",
    switchId: "s-1",
    port: "10GE1/0/2",
    kind: "mac",
    mac: "00:00:5e:00:53:01",
    vid: 30,
    remoteName: "",
    remotePort: "",
    remotePlatform: "",
  },
];
