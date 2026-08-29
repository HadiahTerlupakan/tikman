import { describe, expect, it } from "vitest";
import { clientSteps } from "./vpnClientSteps";

describe("clientSteps", () => {
  it("tells the operator that the MikroTik text is commands, not a file", () => {
    const mikrotik = clientSteps("mikrotik");

    expect(mikrotik.intro).toMatch(/perintah/i);
    expect(mikrotik.steps.join(" ")).toMatch(/terminal/i);
  });

  it("tells the operator that the Linux text is a file, and where it goes", () => {
    const linux = clientSteps("wg-quick");

    expect(linux.intro).toMatch(/berkas/i);
    // The path is the one thing a wrong guess breaks silently: wg-quick reads
    // the interface name from the filename.
    expect(linux.steps.join(" ")).toContain("/etc/wireguard/wg0.conf");
    expect(linux.steps.join(" ")).toContain("wg-quick up wg0");
  });

  it("restricts the file's permissions, since it holds a private key", () => {
    expect(clientSteps("wg-quick").steps.join(" ")).toContain("chmod 600");
  });

  it("gives each format a way to confirm the tunnel came up", () => {
    expect(clientSteps("mikrotik").verify).toContain("last-handshake");
    expect(clientSteps("wg-quick").verify).toContain("wg show");
  });
});
