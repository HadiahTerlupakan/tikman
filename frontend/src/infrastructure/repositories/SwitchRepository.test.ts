import { beforeEach, describe, expect, it, vi } from "vitest";
import { SwitchRepository } from "./SwitchRepository";

const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    post: (...args: unknown[]) => post(...args),
  },
}));

const form = {
  name: "SW-TEST",
  host: "192.0.2.10",
  vendor: "huawei_ce" as const,
  snmpCommunity: "c",
  protocol: "ssh" as const,
  username: "tikman",
  password: "p",
};

describe("SwitchRepository.testConnection", () => {
  beforeEach(() => post.mockReset());

  it("returns the identity of a switch that answered", async () => {
    post.mockResolvedValue({
      status: 200,
      data: { model: "CE6850-48S4Q-EI", version: "V100R006C00" },
    });

    const result = await new SwitchRepository().testConnection(form);

    expect(post.mock.calls[0][0]).toBe("/api/v1/switches/test-connection");
    expect(result).toEqual({
      kind: "identified",
      model: "CE6850-48S4Q-EI",
      version: "V100R006C00",
    });
  });

  // The 409 carries the fingerprint, which the error mapper would drop.
  it("turns an unpinned key into the fingerprint to confirm", async () => {
    post.mockResolvedValue({
      status: 409,
      data: { code: "HOST_KEY_UNPINNED", presentedFingerprint: "SHA256:abc" },
    });

    const result = await new SwitchRepository().testConnection(form);

    expect(result).toEqual({
      kind: "unpinned",
      presentedFingerprint: "SHA256:abc",
    });
    const validate = post.mock.calls[0][2].validateStatus;
    expect(validate(409)).toBe(true);
    expect(validate(502)).toBe(false);
  });
});
