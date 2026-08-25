import type { OltModel } from "./Olt";

export type ZteOnuIdMode = "auto" | "custom";
export type ZteServiceType = "internet";
export type ZteWanMode = "pppoe";

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
  vlanMode: "tag";
  serviceType: ZteServiceType;
  vlanId: number;
  downloadProfile: string;
  uploadProfile: string;
  wanMode: ZteWanMode;
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
