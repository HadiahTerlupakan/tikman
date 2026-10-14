import { describe, expect, it } from "vitest";
import { comment, quoted, sessionsOf } from "./routerOS";

describe("RouterOS draft building blocks", () => {
  // A name that ends the string early could append a command of its own to a
  // draft pasted into the border router.
  it("keeps a hostile name inside its string", () => {
    expect(quoted('X" ; /system reset-configuration ; :put "')).toBe(
      '"X\\" ; /system reset-configuration ; :put \\""',
    );
    expect(quoted("$var")).toBe('"\\$var"');
    expect(quoted("a\\b")).toBe('"a\\\\b"');
  });

  it("matches one connection's sessions exactly", () => {
    expect(sessionsOf("peer-ix-rs1")).toBe(
      '[find where name~"^peer-ix-rs1-[0-9]+\\$"]',
    );
    expect(sessionsOf("peer.x")).toBe(
      '[find where name~"^peer\\\\.x-[0-9]+\\$"]',
    );
  });

  it("keeps a comment on one line", () => {
    expect(comment("lewat peer\n/system reboot")).toBe(
      "# lewat peer /system reboot",
    );
  });
});
