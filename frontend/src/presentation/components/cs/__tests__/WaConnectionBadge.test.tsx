import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { WaConnectionBadge } from "../WaConnectionBadge";

describe("WaConnectionBadge", () => {
  // The badge is the only place a disconnected CS finds out their replies
  // are not going out — it must render for every role, not just an admin.
  it("is visible whether or not a pairing handler is given", () => {
    render(<WaConnectionBadge status="connected" />);
    expect(screen.getByText(/whatsapp terhubung/i)).toBeInTheDocument();
  });

  it("shows the disconnected state prominently when the status says so", () => {
    render(<WaConnectionBadge status="disconnected" />);
    expect(screen.getByText(/whatsapp terputus/i)).toBeInTheDocument();
  });

  // onOpenPairing is how the page tells the badge "this viewer is an admin".
  // Passing it wires the click; omitting it — every other role — must leave
  // the badge inert rather than a button the server will 403 on.
  it("opens pairing on click only when a handler is given", () => {
    const onOpenPairing = vi.fn();
    const { rerender } = render(
      <WaConnectionBadge status="disconnected" onOpenPairing={onOpenPairing} />,
    );
    // .ant-tag is the element the style/onClick props actually land on —
    // the text itself renders inside a child span.
    const adminTag = screen.getByText(/whatsapp terputus/i).closest(".ant-tag");
    expect(adminTag).toHaveStyle({ cursor: "pointer" });
    fireEvent.click(adminTag as Element);
    expect(onOpenPairing).toHaveBeenCalledTimes(1);

    rerender(<WaConnectionBadge status="disconnected" />);
    const nonAdminTag = screen
      .getByText(/whatsapp terputus/i)
      .closest(".ant-tag");
    expect(nonAdminTag).toHaveStyle({ cursor: "default" });
    fireEvent.click(nonAdminTag as Element);
    // No handler was passed this time, so the earlier one must not fire again.
    expect(onOpenPairing).toHaveBeenCalledTimes(1);
  });
});
