import type { ZteWanIpMode } from "./ZteProvisioning";

// An ONU's provisioned service as the OLT currently has it. The password is
// included because reconfiguring the service has to resend it; it is stored
// encrypted and only reaches here over an authenticated session.
export interface OntServiceConfig {
  vlanId: number;
  vlanMode: "tag" | "untag";
  serviceType: "internet" | "bridge";
  tcontProfile: string;
  wanMode: "wan_ip" | "setup_via_ont";
  wanIpMode: ZteWanIpMode | "";
  vlanProfile: string;
  pppoeUsername: string;
  pppoePassword: string;
}
