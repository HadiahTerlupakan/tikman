import type { OltModel } from "./Olt";

export type ZteOnuIdMode = "auto" | "custom";
export type ZteVlanMode = "tag" | "untag";
export type ZteServiceType = "internet" | "bridge";
// Where the WAN is configured, and how an OLT-configured one gets its address.
export type ZteWanMode = "wan_ip" | "setup_via_ont";
export type ZteWanIpMode = "pppoe" | "dhcp" | "static";

export interface ZteGPONRegisterRequest {
  oltId: string;
  card: number;
  pon: number;
  onuIdMode: ZteOnuIdMode;
  onuId: number;
  serialNumber: string;
  onuType: string;
  useVeip: boolean;
  name: string;
  description: string;
  serviceEnabled: boolean;
  vlanMode: ZteVlanMode;
  serviceType: ZteServiceType;
  vlanId: number;
  downloadProfile: string;
  uploadProfile: string;
  wanMode: ZteWanMode;
  wanIpMode: ZteWanIpMode | "";
  vlanProfile: string;
  pppoeUsername: string;
  pppoePassword: string;
  confirm: boolean;
}

export interface ZteProvisionTarget {
  oltId: string;
  oltModel?: OltModel | string;
  card: number;
  pon: number;
  serialNumber: string;
  onuId?: number;
  onuType?: string;
  name?: string;
  description?: string;
}

export interface ZteProvisionResponse {
  jobId: string;
  status: string;
  ontId: string;
  onuId: number;
  commands: string[];
}

export interface ZteProvisionJob {
  jobId: string;
  status: "pending" | "running" | "success" | "failed" | "rolled_back";
  ontId?: string;
  errorMessage?: string;
}

// What the server would send, and the ONU ID it would use.
export interface ZteCommandPreviewResult {
  onuId: number;
  commands: string[];
}
