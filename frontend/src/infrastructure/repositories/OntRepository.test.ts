import { beforeEach, describe, expect, it, vi } from "vitest";
import { OntRepository } from "./OntRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("OntRepository.getTroubled", () => {
  beforeEach(() => {
    get.mockReset();
    get.mockResolvedValue({ data: { data: [], summary: {} } });
  });

  it("names the OLT the way the API reads it", async () => {
    await new OntRepository().getTroubled(24, "olt-1", "offline");

    // The request interceptor decamelizes the body, never the query string, so
    // a camelCase param reaches the server spelled exactly as written and is
    // dropped without a word: the page then showed every OLT under one OLT's
    // name. Every other call site already spells its params this way.
    expect(get.mock.calls[0][1].params).toMatchObject({
      olt_id: "olt-1",
      status: "offline",
      hours: 24,
    });
  });

  it("leaves the OLT out entirely when none is picked", async () => {
    await new OntRepository().getTroubled(24);

    expect(get.mock.calls[0][1].params.olt_id).toBeUndefined();
  });
});
