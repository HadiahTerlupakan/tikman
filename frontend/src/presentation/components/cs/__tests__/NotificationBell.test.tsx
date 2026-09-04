import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { CsConversation } from "@/domain/entities";
import { NotificationBell } from "../NotificationBell";

function conversation(id: string, name: string): CsConversation {
  return {
    id,
    customerPhone: `62811${id}`,
    customerName: name,
    status: "unassigned",
    lastMessageAt: new Date().toISOString(),
    unreadCount: 1,
    hasAvatar: false,
    lastMessageDirection: "in",
    lastMessage: {
      body: `pesan dari ${name}`,
      kind: "text",
      direction: "in",
      at: new Date().toISOString(),
    },
  };
}

const openBell = () =>
  fireEvent.click(
    screen.getByRole("button", { name: /percakapan menunggu dibalas/i }),
  );

describe("NotificationBell", () => {
  it("lists the waiting threads and opens the one clicked", () => {
    const onOpen = vi.fn();
    render(
      <NotificationBell
        conversations={[conversation("1", "Budi"), conversation("2", "Sari")]}
        onOpen={onOpen}
        onSeeAll={vi.fn()}
      />,
    );

    openBell();
    expect(screen.getByText("Budi")).toBeInTheDocument();
    expect(screen.getByText("pesan dari Sari")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Sari"));
    expect(onOpen).toHaveBeenCalledWith("2");
  });

  // The panel is a glance, not a second inbox: past a handful it stops being
  // scannable, and the footer has to say how much it is not showing rather
  // than silently truncating.
  it("caps the list and says how many it left out", () => {
    const many = Array.from({ length: 9 }, (_, i) =>
      conversation(String(i), `Pelanggan ${i}`),
    );
    const onSeeAll = vi.fn();
    render(
      <NotificationBell
        conversations={many}
        onOpen={vi.fn()}
        onSeeAll={onSeeAll}
      />,
    );

    openBell();
    expect(screen.getByText("Pelanggan 5")).toBeInTheDocument();
    expect(screen.queryByText("Pelanggan 6")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText(/lihat semua \(3 lagi\)/i));
    expect(onSeeAll).toHaveBeenCalledTimes(1);
  });

  it("says everything is answered rather than showing an empty list", () => {
    render(
      <NotificationBell
        conversations={[]}
        onOpen={vi.fn()}
        onSeeAll={vi.fn()}
      />,
    );

    openBell();
    expect(screen.getByText(/semua sudah dibalas/i)).toBeInTheDocument();
    expect(screen.getByText("Menunggu dibalas (0)")).toBeInTheDocument();
  });
});
