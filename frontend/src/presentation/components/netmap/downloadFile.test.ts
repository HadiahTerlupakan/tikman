import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadFile } from "./downloadFile";

describe("downloadFile", () => {
  afterEach(() => vi.restoreAllMocks());

  it("hands the browser an object URL through a clicked, then discarded, anchor", () => {
    const objectUrl = "blob:fake-url";
    vi.spyOn(URL, "createObjectURL").mockReturnValue(objectUrl);
    const revoke = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => {});
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});
    const blob = new Blob(["fake kmz"]);

    downloadFile(blob, "peta-jaringan.kmz");

    expect(URL.createObjectURL).toHaveBeenCalledWith(blob);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revoke).toHaveBeenCalledWith(objectUrl);
    // Neither element left the button MapToolbar rendered as the only anchor
    // in the document once the download link served its purpose.
    expect(document.querySelector(`a[href="${objectUrl}"]`)).toBeNull();
  });

  it("names the downloaded file with the filename given, not the blob URL", () => {
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:fake-url");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    let downloadAttr = "";
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      downloadAttr = this.download;
    });

    downloadFile(new Blob(["x"]), "peta-jaringan.kmz");

    expect(downloadAttr).toBe("peta-jaringan.kmz");
  });
});
