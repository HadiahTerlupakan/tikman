import type { ZteWanIpMode } from "./ZteProvisioning";

// An ONU's provisioned service as the OLT currently has it. There is no
// password field: the OLT holds one in clear text and the operator retypes it
// rather than having it travel to a browser.
export interface OntServiceConfig {
  vlanId: number;
  vlanMode: "tag" | "untag";
  serviceType: "internet" | "bridge";
  tcontProfile: string;
  wanMode: "wan_ip" | "setup_via_ont";
  wanIpMode: ZteWanIpMode | "";
  vlanProfile: string;
  pppoeUsername: string;
}
