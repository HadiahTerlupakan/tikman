import { describe, expect, it } from "vitest";
import type { CsLastMessage } from "@/domain/entities";
import { preview } from "./conversationSummary";

const last = (over: Partial<CsLastMessage>): CsLastMessage => ({
  kind: "text",
  body: "",
  direction: "in",
  at: "2026-09-16T03:00:00Z",
  ...over,
});

describe("preview", () => {
  it("names a sticker rather than leaving the line blank", () => {
    expect(preview(last({ kind: "sticker" }))).toBe("Stiker");
  });

  it("still reads a photo with its caption", () => {
    expect(preview(last({ kind: "image", body: "ini modemnya" }))).toBe(
      "Foto · ini modemnya",
    );
  });

  it("says so when a thread has no message yet", () => {
    expect(preview(undefined)).toBe("Belum ada pesan");
  });
});
