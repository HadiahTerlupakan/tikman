import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MessageThread } from "../MessageThread";
import type { CsMessage } from "@/domain/entities";

function message(over: Partial<CsMessage> = {}): CsMessage {
  return {
    id: "m1",
    conversationId: "c1",
    direction: "in",
    kind: "text",
    body: "internet saya mati",
    status: "delivered",
    waTimestamp: "2026-09-03T07:05:00Z",
    ...over,
  } as CsMessage;
}

describe("MessageThread", () => {
  // A customer's screenshot of a whole phone screen, drawn at its natural
  // height, pushed the rest of the conversation off the page.
  it("draws a photo at a fixed thumbnail size, not its natural one", () => {
    render(
      <MessageThread
        messages={[
          message({ kind: "image", body: "", mediaFilename: "ss.jpg" }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    const img = screen.getByAltText("ss.jpg");
    expect(img).toHaveAttribute("width", "220");
    expect(img).toHaveAttribute("height", "160");
  });

  it("offers to open the photo full size", () => {
    render(
      <MessageThread
        messages={[
          message({ kind: "image", body: "", mediaFilename: "ss.jpg" }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText("Lihat penuh")).toBeInTheDocument();
  });

  // A reply that was refused must say why and offer a way back, or it reads as
  // sent and the customer is left waiting on an answer that never went.
  it("shows why a reply failed and lets it be sent again", async () => {
    const onRetry = vi.fn();
    render(
      <MessageThread
        messages={[
          message({
            direction: "out",
            status: "failed",
            body: "sudah kami cek",
            failReason: "nomor tidak terdaftar di WhatsApp",
          }),
        ]}
        onRetry={onRetry}
      />,
    );

    expect(
      screen.getByText(/nomor tidak terdaftar di WhatsApp/i),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    expect(onRetry).toHaveBeenCalledWith("sudah kami cek");
  });

  // This outbox is at-least-once: a queued reply has not been sent, and a tick
  // saying otherwise would be a lie a CS acts on.
  it("marks a queued reply as waiting rather than sent", () => {
    render(
      <MessageThread
        messages={[message({ direction: "out", status: "queued" })]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("Menunggu dikirim")).toBeInTheDocument();
  });

  it("marks a read reply as read", () => {
    render(
      <MessageThread
        messages={[message({ direction: "out", status: "read" })]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("Dibaca")).toBeInTheDocument();
  });

  // Ticks belong to replies only: a customer's message has no delivery state
  // this inbox knows about.
  it("puts no delivery mark on a customer's message", () => {
    render(
      <MessageThread
        messages={[message({ direction: "in", status: "delivered" })]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.queryByLabelText("Sampai di HP pelanggan")).toBeNull();
  });

  // antd wraps the image in an inline-block element, so a caption — a sibling
  // span — sat beside the photo and ran off the edge of the bubble instead of
  // sitting under it.
  it("puts a caption beneath the photo, not beside it", () => {
    const { container } = render(
      <MessageThread
        messages={[
          message({
            kind: "image",
            body: "struk pembayaran",
            mediaFilename: "struk.jpg",
          }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText("struk pembayaran")).toBeInTheDocument();
    expect(container.querySelector(".ant-image")).toHaveStyle({
      display: "block",
    });
  });
});
