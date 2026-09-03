import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MessageThread } from "../MessageThread";
import type { CsMessage } from "@/domain/entities";

function message(id: string, body: string): CsMessage {
  return {
    id,
    conversationId: "c1",
    direction: "in",
    kind: "text",
    body,
    status: "delivered",
    waTimestamp: "2026-09-03T10:00:00Z",
    createdAt: "2026-09-03T10:00:00Z",
  } as CsMessage;
}

function draw(props: Partial<Parameters<typeof MessageThread>[0]> = {}) {
  return render(
    <MessageThread
      messages={[message("m1", "internet mati")]}
      onRetry={vi.fn()}
      {...props}
    />,
  );
}

const deleteButton = () =>
  screen.queryByRole("button", { name: "Hapus pesan ini" });

describe("deleting a message from the thread", () => {
  // Offering the gesture to someone the API will refuse is the worse of the
  // two: they act, nothing happens, and nothing says why.
  it("offers no delete button to someone who may not delete", () => {
    draw();
    expect(deleteButton()).not.toBeInTheDocument();
  });

  // A button belonging to the wrong bubble deletes the wrong message, and the
  // CS finds out by reading a thread that has lost the line they meant to keep.
  // History arrives newest first and is drawn newest last, so the bottom
  // bubble — and the last delete button — is messages[0].
  it("hands back the message whose own button was pressed", async () => {
    const onDelete = vi.fn();
    const newest = message("m1", "internet mati");
    const older = message("m2", "sudah dari kemarin");
    draw({ messages: [newest, older], onDelete });

    const buttons = screen.getAllByRole("button", { name: "Hapus pesan ini" });
    expect(buttons).toHaveLength(2);

    await userEvent.click(buttons[1]);
    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    expect(onDelete).toHaveBeenCalledWith(
      expect.objectContaining({ id: "m1" }),
    );
  });

  it("draws the oldest message first, so the newest reads last", () => {
    draw({
      messages: [message("m1", "internet mati"), message("m2", "dari kemarin")],
      onDelete: vi.fn(),
    });

    const bubbles = screen.getAllByText(/internet mati|dari kemarin/);
    expect(bubbles[0]).toHaveTextContent("dari kemarin");
    expect(bubbles[1]).toHaveTextContent("internet mati");
  });

  // Nothing may go on one click. The button opens a confirmation, and closing
  // it must leave the message where it was.
  it("removes nothing when the confirmation is cancelled", async () => {
    const onDelete = vi.fn();
    draw({ onDelete });

    await userEvent.click(deleteButton()!);
    await userEvent.click(screen.getByRole("button", { name: "Batal" }));

    expect(onDelete).not.toHaveBeenCalled();
  });

  // It removes the copy held here and nothing else. A CS who believes it
  // reaches the customer's phone will tell the customer so.
  it("says plainly that the customer still has their copy", async () => {
    draw({ onDelete: vi.fn() });
    await userEvent.click(deleteButton()!);

    expect(
      screen.getByText(/pesan di hp pelanggan tetap ada/i),
    ).toBeInTheDocument();
  });
});
