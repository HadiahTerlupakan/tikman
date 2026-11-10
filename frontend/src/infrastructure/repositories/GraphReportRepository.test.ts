import { beforeEach, describe, expect, it, vi } from "vitest";
import { GraphReportRepository } from "./GraphReportRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

const jsonBlob = (body: unknown) =>
  new Blob([JSON.stringify(body)], { type: "application/json" });

describe("GraphReportRepository.graph", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("asks for the port by device_id and if_name and returns the PNG", async () => {
    const png = new Blob(["png"], { type: "image/png" });
    get.mockResolvedValue({ status: 200, data: png });

    const blob = await new GraphReportRepository().graph(7, "ether 1/1");

    expect(blob).toBe(png);
    const [url, config] = get.mock.calls[0];
    expect(url).toBe("/api/v1/graph-reports/librenms/graph");
    expect(config.params).toEqual({ device_id: 7, if_name: "ether 1/1" });
    expect(config.responseType).toBe("blob");
  });

  it("reports the server's own reason, which arrives inside a blob", async () => {
    get.mockResolvedValue({
      status: 400,
      data: jsonBlob({ error: "LibreNMS belum diatur di Pengaturan" }),
    });

    await expect(new GraphReportRepository().graph(7, "sfp1")).rejects.toThrow(
      "LibreNMS belum diatur di Pengaturan",
    );
  });

  it("falls back to the status when the error body is not JSON", async () => {
    get.mockResolvedValue({ status: 502, data: new Blob(["Bad Gateway"]) });

    await expect(new GraphReportRepository().graph(7, "sfp1")).rejects.toThrow(
      "HTTP 502",
    );
  });
});
