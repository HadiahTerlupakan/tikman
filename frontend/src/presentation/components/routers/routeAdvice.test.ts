import { describe, expect, it } from "vitest";
import type { RouteAdvice } from "@/domain/entities";
import { adviceText, mainInbound, needsAction } from "./routeAdvice";

const base: RouteAdvice = {
  prefix: "198.51.100.0/24",
  asn: 64496,
  name: "EXAMPLE-NET",
  downloadBytes: 1000,
  uploadBytes: 10,
  status: "optimal",
  active: {
    peer: "peer-cdn",
    remoteAs: 64511,
    transit: false,
    asPath: "64511,64520,64521,64496",
    pathLength: 4,
    localPref: 600,
    inputFilter: "CDN_IN",
  },
  alternatives: [
    {
      peer: "peer-small-ix",
      remoteAs: 64512,
      transit: false,
      asPath: "64512,64496",
      pathLength: 2,
      localPref: 300,
      inputFilter: "SMALLIX_IN",
    },
  ],
  inbound: null,
  suggested: "peer-small-ix",
  checkedAt: "2026-10-14T03:00:00Z",
};

describe("route advice helpers", () => {
  it("finds the peer most download came from", () => {
    expect(
      mainInbound([
        { peer: "peer-transit", transit: true, bytes: 800 },
        { peer: "peer-ix", transit: false, bytes: 200 },
      ]),
    ).toEqual({ peer: "peer-transit", share: 0.8 });
    expect(mainInbound(null)).toBeNull();
    expect(mainInbound([{ peer: "x", transit: false, bytes: 0 }])).toBeNull();
  });

  it("needs action for every status but optimal", () => {
    expect(needsAction(base)).toBe(false);
    expect(needsAction({ ...base, status: "transit_only" })).toBe(true);
  });

  it("explains each status in its own terms", () => {
    expect(adviceText({ ...base, status: "transit_only" })).toContain(
      "peering langsung atau cache dengan EXAMPLE-NET (AS64496)",
    );
    expect(adviceText({ ...base, status: "inbound_via_transit" })).toContain(
      "hanya mengatur trafik keluar",
    );
    expect(adviceText({ ...base, status: "shorter_outbound" })).toContain(
      "Lewat peer-small-ix AS path-nya 2, lebih pendek dari jalur aktif lewat peer-cdn (4)",
    );
  });
});
