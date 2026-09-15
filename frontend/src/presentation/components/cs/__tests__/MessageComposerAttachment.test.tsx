import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MessageComposer } from "../MessageComposer";
import type { CsConversation } from "@/domain/entities";

const conversation = {
  id: "c1",
  customerPhone: "628111222333",
  customerName: "Budi",
  status: "open" as const,
  lastMessageAt: "2026-09-15T01:00:00Z",
  unreadCount: 0,
  hasAvatar: false,
  assignedUserId: "me",
} as CsConversation;

const screenshot = () => new File(["png"], "image.png", { type: "image/png" });

function composer(thread: CsConversation = conversation) {
  const onAttach = vi.fn().mockResolvedValue(true);
  const props = {
    currentUserId: "me",
    holderName: "Saya",
    onSend: vi.fn().mockResolvedValue(true),
    onTakeOver: vi.fn(),
    onTypingChange: vi.fn(),
    onAttach,
  };
  const view = render(<MessageComposer conversation={thread} {...props} />);
  const rerenderWith = (next: CsConversation) =>
    view.rerender(<MessageComposer conversation={next} {...props} />);
  return {
    onAttach,
    rerenderWith,
    box: screen.getByPlaceholderText("Tulis balasan"),
  };
}

/** Answers whether the browser was left to paste on its own. */
function paste(target: HTMLElement, files: File[]) {
  return fireEvent.paste(target, {
    clipboardData: { files, getData: () => "" },
  });
}

const sendButton = () => screen.getByRole("button", { name: /^kirim$/i });

beforeEach(() => {
  // jsdom has no blob URLs; the thumbnail only needs one to point at.
  Object.assign(URL, {
    createObjectURL: vi.fn(() => "blob:preview"),
    revokeObjectURL: vi.fn(),
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("attaching in the composer", () => {
  // A screenshot pasted into the wrong thread must be caught on this screen,
  // not on the customer's phone.
  it("shows a pasted image before anything is sent", () => {
    const { box, onAttach } = composer();

    paste(box, [screenshot()]);

    expect(screen.getByRole("img", { name: "image.png" })).toHaveAttribute(
      "src",
      "blob:preview",
    );
    expect(onAttach).not.toHaveBeenCalled();
  });

  it("sends the pasted image with the caption typed in the box", async () => {
    const { box, onAttach } = composer();
    const image = screenshot();

    paste(box, [image]);
    await userEvent.type(box, "ini fotonya");
    await userEvent.click(sendButton());

    expect(onAttach).toHaveBeenCalledWith(image, "ini fotonya");
    await waitFor(() => expect(box).toHaveValue(""));
    expect(screen.queryByText("image.png")).not.toBeInTheDocument();
  });

  it("sends a pasted image on Enter, the way a reply goes", async () => {
    const { box, onAttach } = composer();
    const image = screenshot();

    paste(box, [image]);
    await userEvent.type(box, "{Enter}");

    expect(onAttach).toHaveBeenCalledWith(image, "");
    await waitFor(() =>
      expect(screen.queryByText("image.png")).not.toBeInTheDocument(),
    );
  });

  it("lets the CS take a pasted image back without sending it", async () => {
    const { box, onAttach } = composer();

    paste(box, [screenshot()]);
    await userEvent.click(
      screen.getByRole("button", { name: "Batalkan lampiran" }),
    );

    expect(screen.queryByText("image.png")).not.toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
    expect(onAttach).not.toHaveBeenCalled();
  });

  it("leaves pasted text to the box", () => {
    const { box } = composer();

    expect(paste(box, [])).toBe(true);
    expect(
      screen.queryByRole("button", { name: "Batalkan lampiran" }),
    ).not.toBeInTheDocument();
  });

  it("refuses a pasted file WhatsApp will not take", async () => {
    const { box, onAttach } = composer();

    paste(box, [new File(["zip"], "arsip.zip", { type: "application/zip" })]);

    expect(
      await screen.findByText(/tidak bisa dikirim lewat WhatsApp/),
    ).toBeInTheDocument();
    expect(screen.queryByText("arsip.zip")).not.toBeInTheDocument();
    expect(onAttach).not.toHaveBeenCalled();
  });

  // The paperclip used to send the moment a file was picked, with whatever
  // happened to be typed as its caption.
  it("holds a picked file for review instead of sending it at once", async () => {
    const { onAttach } = composer();
    const report = new File(["pdf"], "laporan.pdf", {
      type: "application/pdf",
    });

    fireEvent.change(
      document.body.querySelector('input[type="file"]') as HTMLInputElement,
      { target: { files: [report] } },
    );

    expect(await screen.findByText("laporan.pdf")).toBeInTheDocument();
    expect(onAttach).not.toHaveBeenCalled();

    await userEvent.click(sendButton());
    expect(onAttach).toHaveBeenCalledWith(report, "");
  });

  // Carrying it over would send one customer's attachment to the next.
  it("drops a waiting attachment when the CS opens another conversation", () => {
    const { box, rerenderWith } = composer();

    paste(box, [screenshot()]);
    expect(screen.getByText("image.png")).toBeInTheDocument();
    rerenderWith({ ...conversation, id: "c2" });

    expect(screen.queryByText("image.png")).not.toBeInTheDocument();
  });
});
