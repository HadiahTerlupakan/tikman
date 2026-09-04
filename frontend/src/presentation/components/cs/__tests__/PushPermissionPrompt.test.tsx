import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { PushPermissionPrompt } from "../PushPermissionPrompt";

describe("PushPermissionPrompt", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("asks on open when permission has not been decided", () => {
    render(
      <PushPermissionPrompt
        permission="default"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );
    expect(screen.getByText(/aktifkan notifikasi\?/i)).toBeInTheDocument();
  });

  it("hands the click straight to the enable flow", () => {
    const onEnable = vi.fn();
    render(
      <PushPermissionPrompt
        permission="default"
        requesting={false}
        onEnable={onEnable}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^aktifkan$/i }));
    expect(onEnable).toHaveBeenCalledTimes(1);
  });

  // The browser's own prompt can only be answered once, so a CS who is busy
  // must be able to put it off without spending it.
  it("stops asking for the rest of the session once put off", () => {
    const { unmount } = render(
      <PushPermissionPrompt
        permission="default"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /nanti saja/i }));
    unmount();

    render(
      <PushPermissionPrompt
        permission="default"
        requesting={false}
        onEnable={vi.fn()}
      />,
    );
    expect(
      screen.queryByText(/aktifkan notifikasi\?/i),
    ).not.toBeInTheDocument();
  });

  it.each(["granted", "denied", "unsupported"] as const)(
    "stays quiet when permission is already %s",
    (permission) => {
      render(
        <PushPermissionPrompt
          permission={permission}
          requesting={false}
          onEnable={vi.fn()}
        />,
      );
      expect(
        screen.queryByText(/aktifkan notifikasi\?/i),
      ).not.toBeInTheDocument();
    },
  );
});
