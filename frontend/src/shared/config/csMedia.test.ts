import { describe, expect, it } from "vitest";
import { CS_MEDIA_MAX_BYTES, attachmentRejection } from "./csMedia";

function fileOf(type: string, size: number): File {
  const file = new File(["x"], "lampiran", { type });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

describe("attachmentRejection", () => {
  it("passes a type the server's allowlist accepts", () => {
    expect(attachmentRejection(fileOf("image/jpeg", 1024))).toBeNull();
  });

  // The server refuses html/svg outright, because it serves attachments back
  // from the API's own origin. Learning that here costs no round trip.
  it("refuses a type the server would refuse", () => {
    expect(attachmentRejection(fileOf("text/html", 1024))).toMatch(
      /tidak bisa dikirim/i,
    );
  });

  it("refuses a file past the size the upload endpoint caps at", () => {
    expect(
      attachmentRejection(fileOf("image/jpeg", CS_MEDIA_MAX_BYTES + 1)),
    ).toMatch(/melebihi batas/i);
  });
});
