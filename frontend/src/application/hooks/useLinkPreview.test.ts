import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

// vi.mock is hoisted above every const in the file, so the spy has to be
// created inside vi.hoisted or the factory closes over a binding that does
// not exist yet.
const { getLinkPreview } = vi.hoisted(() => ({ getLinkPreview: vi.fn() }));

vi.mock("@/infrastructure/repositories", () => ({
  CsRepository: class {
    getLinkPreview = getLinkPreview;
  },
}));

import { useLinkPreview } from "./useLinkPreview";

describe("useLinkPreview", () => {
  beforeEach(() => getLinkPreview.mockReset());

  // Most keystrokes contain no link. Asking the server about every one of them
  // would make the composer chatty for nothing.
  it("does not ask the server about a draft with no link", async () => {
    renderHook(() => useLinkPreview("halo pak sudah dicek ya"));

    await act(() => new Promise((r) => setTimeout(r, 700)));

    expect(getLinkPreview).not.toHaveBeenCalled();
  });

  it("returns the card once a link appears", async () => {
    getLinkPreview.mockResolvedValue({
      url: "https://example.com",
      title: "Judul",
    });

    const { result } = renderHook(() =>
      useLinkPreview("lihat https://example.com"),
    );

    await waitFor(() => expect(result.current.preview?.title).toBe("Judul"));
  });

  // A page with nothing worth showing is a normal answer, not an error state.
  it("holds no card when the server reports none", async () => {
    getLinkPreview.mockResolvedValue(null);

    const { result } = renderHook(() =>
      useLinkPreview("lihat https://example.com"),
    );

    await act(() => new Promise((r) => setTimeout(r, 700)));
    expect(result.current.preview).toBeNull();
  });

  // A CS who dismisses the card must not have it reappear on the next
  // keystroke of the same draft.
  it("stays dismissed until the link changes", async () => {
    getLinkPreview.mockResolvedValue({
      url: "https://example.com",
      title: "Judul",
    });
    const { result, rerender } = renderHook(({ t }) => useLinkPreview(t), {
      initialProps: { t: "lihat https://example.com" },
    });
    await waitFor(() => expect(result.current.preview).not.toBeNull());

    act(() => result.current.dismiss());
    rerender({ t: "lihat https://example.com sekarang" });

    await act(() => new Promise((r) => setTimeout(r, 700)));
    expect(result.current.preview).toBeNull();
  });
});
