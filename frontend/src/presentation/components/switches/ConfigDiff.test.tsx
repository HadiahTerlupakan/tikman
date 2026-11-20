import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { ConfigDiff } from "./ConfigDiff";

const DIFF = [
  "--- 2026-11-10T01:00:00Z",
  "+++ 2026-11-11T01:00:00Z",
  "@@ -1,3 +1,3 @@",
  " sysname SW-TEST",
  "-vlan 20",
  "+vlan 30",
].join("\n");

describe("ConfigDiff", () => {
  it("tags added, removed and context lines", () => {
    const { container } = render(<ConfigDiff diff={DIFF} />);
    const kind = (text: string) =>
      [...container.querySelectorAll("[data-kind]")]
        .find((el) => el.textContent === text)
        ?.getAttribute("data-kind");

    expect(kind("+vlan 30")).toBe("add");
    expect(kind("-vlan 20")).toBe("del");
    expect(kind(" sysname SW-TEST")).toBe("context");
    expect(kind("@@ -1,3 +1,3 @@")).toBe("hunk");
    expect(kind("+++ 2026-11-11T01:00:00Z")).toBe("header");
  });

  it("says so when the two backups are the same", () => {
    const { container } = render(<ConfigDiff diff="" />);
    expect(container.textContent).toContain("Tidak ada perbedaan");
  });
});
