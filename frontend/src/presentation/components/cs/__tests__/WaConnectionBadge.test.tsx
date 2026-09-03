import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { WaConnectionBadge } from "../WaConnectionBadge";
import type { WaAccount, WaAccountStatus } from "@/domain/entities";

function account(id: string, status: WaAccountStatus): WaAccount {
  return {
    id,
    label: `CS ${id}`,
    status,
    createdAt: "",
    updatedAt: "",
  };
}

describe("WaConnectionBadge", () => {
  // The badge is the only place a CS finds out their replies are not going
  // out — it must render for every role, not just an admin.
  it("says every number is up when they are", () => {
    render(
      <WaConnectionBadge
        accounts={[account("1", "connected"), account("2", "connected")]}
        stream={{}}
      />,
    );
    expect(screen.getByText("WhatsApp Terhubung (2)")).toBeInTheDocument();
  });

  // One number down out of six is still a problem, and hiding it behind a
  // green tag is how replies pile up unsent on that number's threads.
  it("counts the numbers that are down rather than hiding them behind the rest", () => {
    render(
      <WaConnectionBadge
        accounts={[
          account("1", "connected"),
          account("2", "disconnected"),
          account("3", "connected"),
        ]}
        stream={{}}
      />,
    );
    expect(screen.getByText("1 dari 3 nomor terputus")).toBeInTheDocument();
  });

  // The account list is fetched once at page load; a number that dropped
  // since then is only known from the stream.
  it("prefers what the stream has seen over the list it loaded with", () => {
    render(
      <WaConnectionBadge
        accounts={[account("1", "connected")]}
        stream={{ "1": { waStatus: "disconnected" } }}
      />,
    );
    expect(screen.getByText("1 dari 1 nomor terputus")).toBeInTheDocument();
  });

  // onOpenNumbers is how the page says "this viewer is an admin". Omitting it
  // must leave the badge inert rather than a button the server will refuse.
  it("opens the numbers panel on click only when a handler is given", () => {
    const onOpenNumbers = vi.fn();
    const { rerender } = render(
      <WaConnectionBadge
        accounts={[account("1", "disconnected")]}
        stream={{}}
        onOpenNumbers={onOpenNumbers}
      />,
    );
    // .ant-tag is the element the style/onClick props actually land on —
    // the text itself renders inside a child span.
    const adminTag = screen.getByText(/terputus/i).closest(".ant-tag");
    expect(adminTag).toHaveStyle({ cursor: "pointer" });
    fireEvent.click(adminTag as Element);
    expect(onOpenNumbers).toHaveBeenCalledTimes(1);

    rerender(
      <WaConnectionBadge
        accounts={[account("1", "disconnected")]}
        stream={{}}
      />,
    );
    const nonAdminTag = screen.getByText(/terputus/i).closest(".ant-tag");
    expect(nonAdminTag).toHaveStyle({ cursor: "default" });
    fireEvent.click(nonAdminTag as Element);
    expect(onOpenNumbers).toHaveBeenCalledTimes(1);
  });
  // Deleting the last number leaves a loaded, empty list. The badge is the
  // only door to the numbers panel, so a dead tag here locks an admin out of
  // pairing a replacement with no way back.
  it("stays a door into the numbers panel when no numbers are left", () => {
    const onOpenNumbers = vi.fn();
    render(
      <WaConnectionBadge
        accounts={[]}
        stream={{}}
        onOpenNumbers={onOpenNumbers}
      />,
    );
    const tag = screen.getByText(/belum ada nomor/i).closest(".ant-tag");
    expect(tag).toHaveStyle({ cursor: "pointer" });
    fireEvent.click(tag as Element);
    expect(onOpenNumbers).toHaveBeenCalledTimes(1);
  });

  // A list that has not arrived is not a list with nothing in it: claiming
  // "no numbers" while the fetch is in flight would tell an admin to pair a
  // number they already have.
  it("says it is still checking only until the list arrives", () => {
    render(<WaConnectionBadge stream={{}} />);
    expect(screen.getByText("Memeriksa koneksi WhatsApp…")).toBeInTheDocument();
  });
});
