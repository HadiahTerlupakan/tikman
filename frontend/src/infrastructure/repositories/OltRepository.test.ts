import { beforeEach, describe, expect, it, vi } from "vitest";
import { OltRepository } from "./OltRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("OltRepository.getUnconfiguredOnus", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("unwraps the envelope into the ONU list", async () => {
    get.mockResolvedValue({
      data: {
        oltId: "olt-1",
        total: 1,
        data: [
          {
            slot: 3,
            port: 1,
            serialNumber: "HWTCB403E8A0",
            deviceType: "HG8245H5",
            softwareVersion: "V5R019C00S105",
          },
        ],
      },
    });

    const onus = await new OltRepository().getUnconfiguredOnus("olt-1");

    expect(get).toHaveBeenCalledWith("/api/v1/olts/olt-1/unconfigured-onus");
    expect(onus).toEqual([
      {
        slot: 3,
        port: 1,
        serialNumber: "HWTCB403E8A0",
        deviceType: "HG8245H5",
        softwareVersion: "V5R019C00S105",
      },
    ]);
  });

  it("returns an empty list when the OLT reports no unconfigured ONUs", async () => {
    get.mockResolvedValue({ data: { oltId: "olt-1", total: 0, data: [] } });

    await expect(
      new OltRepository().getUnconfiguredOnus("olt-1"),
    ).resolves.toEqual([]);
  });

  it("returns an empty list when the payload omits the data array", async () => {
    get.mockResolvedValue({ data: { oltId: "olt-1", total: 0 } });

    await expect(
      new OltRepository().getUnconfiguredOnus("olt-1"),
    ).resolves.toEqual([]);
  });
});
