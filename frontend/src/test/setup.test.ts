import { describe, it, expect } from "vitest";

describe("getComputedStyle shim", () => {
  it("swallows the selector-parse failure antd's Select rules provoke", () => {
    const style = document.createElement("style");
    style.textContent =
      "*+.ant-select-item-option-selected:not(.ant-select-item-option-disabled)) { color: red; }";
    document.head.appendChild(style);
    const element = document.createElement("div");
    document.body.appendChild(element);

    expect(() => window.getComputedStyle(element)).not.toThrow();
  });

  it("lets a real TypeError through instead of hiding it", () => {
    expect(() => window.getComputedStyle(null as unknown as Element)).toThrow(
      "not of type 'Element'",
    );
  });
});
