import { describe, expect, it } from "vitest";
import { isBinaryResponseType } from "./apiClient";

// humps.camelizeKeys walks an object's own enumerable properties. A Blob (or
// an ArrayBuffer) has none of its own - size/type live on the prototype - so
// camelizeKeys silently turns a binary response into `{}` unless the response
// interceptor is told to leave it alone. This guards the check that skips it.
describe("isBinaryResponseType", () => {
  it("treats a blob response as binary", () => {
    expect(isBinaryResponseType("blob")).toBe(true);
  });

  it("treats an arraybuffer response as binary", () => {
    expect(isBinaryResponseType("arraybuffer")).toBe(true);
  });

  it("treats an ordinary JSON response as not binary", () => {
    expect(isBinaryResponseType("json")).toBe(false);
    expect(isBinaryResponseType(undefined)).toBe(false);
  });
});
