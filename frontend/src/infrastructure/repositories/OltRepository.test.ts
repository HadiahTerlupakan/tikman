import { beforeEach, describe, expect, it, vi } from "vitest";
import { OltRepository } from "./OltRepository";

const get = vi.fn();
const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
    post: (...args: unknown[]) => post(...args),
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

describe("OltRepository.refreshSystem", () => {
  beforeEach(() => {
    post.mockReset();
    post.mockResolvedValue({ data: { data: {} } });
  });

  it("posts to the OLT's own refresh route", async () => {
    await new OltRepository().refreshSystem(
      "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b",
    );

    expect(post).toHaveBeenCalledWith(
      "/api/v1/olts/6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b/system/refresh",
    );
  });

  // The id is the configuration page's decoded :id route param, so a link to
  // /olts/..%2Fadmin%2Fseed-events%3F/configuration hands it "../" and "?".
  it("keeps a crafted id inside the refresh route", async () => {
    await new OltRepository().refreshSystem("../admin/seed-events?");

    const [path] = post.mock.calls[0];
    const resolved = new URL(path, "https://tikman.example");
    expect(resolved.pathname).toBe(
      "/api/v1/olts/..%2Fadmin%2Fseed-events%3F/system/refresh",
    );
    expect(resolved.search).toBe("");
  });
});
