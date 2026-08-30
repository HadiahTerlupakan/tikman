import { beforeEach, describe, expect, it, vi } from "vitest";
import { camelizeKeys } from "humps";
import { SettingRepository } from "./SettingRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("SettingRepository.browser", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("keys the value by the real setting name after the response passes through humps", async () => {
    // apiClient's response interceptor runs every body through humps'
    // camelizeKeys before a repository ever sees it. Hand-building an
    // already-camelCase fixture here would prove nothing about that
    // boundary, so this pipes the raw server shape through the same
    // camelizeKeys call the interceptor uses. A map keyed by setting name
    // ("google_maps_api_key") would come back keyed "googleMapsApiKey" and
    // silently break every lookup by GOOGLE_MAPS_API_KEY; the {values:
    // [{name, value}]} list must survive it untouched.
    get.mockResolvedValue({
      data: camelizeKeys({
        values: [{ name: "google_maps_api_key", value: "AIzaSyTESTKEY123" }],
      }),
    });

    const values = await new SettingRepository().browser();

    expect(values).toEqual({ google_maps_api_key: "AIzaSyTESTKEY123" });
  });

  it("returns an empty object when nothing is configured, not an error", async () => {
    get.mockResolvedValue({ data: camelizeKeys({ values: [] }) });

    const values = await new SettingRepository().browser();

    expect(values).toEqual({});
  });
});
