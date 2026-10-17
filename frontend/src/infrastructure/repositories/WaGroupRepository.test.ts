import { beforeEach, describe, expect, it, vi } from "vitest";
import { WaGroupRepository } from "./WaGroupRepository";

const get = vi.fn();
const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
    post: (...args: unknown[]) => post(...args),
  },
}));

describe("WaGroupRepository", () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
  });

  // Query params skip the request interceptor's decamelize, so they have to
  // be written in the server's own spelling.
  it("filters the list by number in snake_case and drops an empty search", async () => {
    get.mockResolvedValue({ data: { data: [] } });

    await new WaGroupRepository().list({ waAccountId: "acc1", search: "" });

    expect(get).toHaveBeenCalledWith("/api/v1/cs/groups", {
      params: { wa_account_id: "acc1", search: undefined },
    });
  });

  it("reads a group with no members listed as an empty member list", async () => {
    get.mockResolvedValue({ data: { data: { group: { id: "g1" } } } });

    const detail = await new WaGroupRepository().get("g1");

    expect(detail).toEqual({ group: { id: "g1" }, participants: [] });
  });

  it("carries the quoted message of a group upload in the query string", async () => {
    post.mockResolvedValue({ data: { data: { id: "m1" } } });
    const file = new File(["x"], "foto.jpg", { type: "image/jpeg" });

    await new WaGroupRepository().sendMedia("g1", file, "lihat", "m0");

    const [url, form] = post.mock.calls[0];
    expect(url).toBe("/api/v1/cs/groups/g1/media?reply_to_id=m0");
    expect((form as FormData).get("caption")).toBe("lihat");
  });
});
