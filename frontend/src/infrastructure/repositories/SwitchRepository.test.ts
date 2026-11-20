import { beforeEach, describe, expect, it, vi } from "vitest";
import { SwitchRepository } from "./SwitchRepository";

const post = vi.fn();
const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
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

describe("SwitchRepository.backupDiff", () => {
  it("asks for the two backups and returns the diff text", async () => {
    get.mockResolvedValue({ data: { diff: "-a\n+b" } });

    const diff = await new SwitchRepository().backupDiff("s-1", "b1", "b2");

    expect(diff).toBe("-a\n+b");
    expect(get).toHaveBeenCalledWith("/api/v1/switches/s-1/backups/diff", {
      params: { from: "b1", to: "b2" },
    });
  });
});

describe("SwitchRepository.trustHostKey", () => {
  beforeEach(() => post.mockReset());

  // The server pins only the fingerprint the admin saw, so it must be sent.
  it("sends the fingerprint being trusted", async () => {
    post.mockResolvedValue({ status: 200, data: { id: "s-1" } });

    await new SwitchRepository().trustHostKey("s-1", "SHA256:shown");

    expect(post).toHaveBeenCalledWith("/api/v1/switches/s-1/trust-host-key", {
      fingerprint: "SHA256:shown",
    });
  });
});
