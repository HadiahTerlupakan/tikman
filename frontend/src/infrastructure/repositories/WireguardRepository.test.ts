import { beforeEach, describe, expect, it, vi } from "vitest";
import { WireguardRepository } from "./WireguardRepository";

const get = vi.fn();
const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
    post: (...args: unknown[]) => post(...args),
  },
}));

describe("WireguardRepository", () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
  });

  it("requests the peer config in the format the caller asked for", async () => {
    get.mockResolvedValue({
      data: { format: "mikrotik", config: "/interface" },
    });

    const config = await new WireguardRepository().getPeerConfig(
      "peer-1",
      "mikrotik",
    );

    expect(get).toHaveBeenCalledWith(
      "/api/v1/wireguard/peers/peer-1/config?format=mikrotik",
    );
    expect(config.format).toBe("mikrotik");
  });

  it("reads the suggested subnets for a site", async () => {
    get.mockResolvedValue({ data: { subnets: ["10.10.10.0/24"] } });

    const subnets = await new WireguardRepository().getSuggestedSubnets(
      "site-1",
    );

    expect(get).toHaveBeenCalledWith(
      "/api/v1/wireguard/sites/site-1/suggested-subnets",
    );
    expect(subnets).toEqual(["10.10.10.0/24"]);
  });
});
