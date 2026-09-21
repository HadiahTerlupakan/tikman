import { describe, expect, it } from "vitest";
import type { AxiosResponse, InternalAxiosRequestConfig } from "axios";
import { apiClient, isBinaryResponseType } from "./apiClient";

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

// fakeAdapter stands in for the network: a real axios adapter, passed per
// request, so a call through the real apiClient instance still runs its
// real request and response interceptors without an actual HTTP call.
function fakeAdapter(data: unknown) {
  return (config: InternalAxiosRequestConfig): Promise<AxiosResponse> =>
    Promise.resolve({
      data,
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    });
}

// isBinaryResponseType's own tests above only pin the three-line predicate
// in isolation - deleting its one use inside the response interceptor
// (apiClient.ts) would leave all three green, since nothing sent an actual
// Blob through the real apiClient instance. This pushes a real Blob through
// the real instance instead, the same shape MappingRepository.exportKmz
// actually sends.
describe("apiClient response interceptor", () => {
  it("leaves a blob response untouched rather than camelizing it into {}", async () => {
    const blob = new Blob(["isi kmz"], {
      type: "application/vnd.google-earth.kmz",
    });

    const response = await apiClient.get("/fake", {
      responseType: "blob",
      adapter: fakeAdapter(blob),
    });

    expect(response.data).toBe(blob);
  });

  // The paired negative case: an interceptor that skipped every response
  // (not only binary ones) would also pass the test above.
  it("still camelizes an ordinary JSON response", async () => {
    const response = await apiClient.get("/fake", {
      adapter: fakeAdapter({ node_id: "ODP-01" }),
    });

    expect(response.data).toEqual({ nodeId: "ODP-01" });
  });
});
