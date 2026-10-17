import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupComposer } from "../GroupComposer";
import { waGroup } from "./fixtures";

const joko = {
  jid: "55511122233344@lid",
  phone: "",
  name: "Pak Joko",
  pushName: "",
  hasAvatar: false,
  role: "member" as const,
  isSelf: false,
};

const props = {
  participants: [joko],
  onAttach: vi.fn(),
  sending: false,
  attaching: false,
  onCancelReply: vi.fn(),
};

describe("GroupComposer", () => {
  it("replaces the box with the reason when only admins may send", () => {
    render(
      <GroupComposer
        {...props}
        group={waGroup({ announce: true })}
        onSend={vi.fn()}
      />,
    );

    expect(screen.queryByPlaceholderText(/Tulis pesan ke grup/)).toBeNull();
    expect(screen.getByText(/Hanya admin grup/)).toBeInTheDocument();
  });

  it("sends what was typed and clears the box once it left", async () => {
    const onSend = vi.fn().mockResolvedValue(true);
    render(<GroupComposer {...props} group={waGroup()} onSend={onSend} />);

    const box = screen.getByPlaceholderText(/Tulis pesan ke grup/);
    await userEvent.type(box, "siap meluncur{Enter}");

    expect(onSend).toHaveBeenCalledWith("siap meluncur", []);
    expect(box).toHaveValue("");
  });

  // The text alone notifies nobody: WhatsApp needs the address written in
  // and the member's JID listed beside it.
  it("sends a typed mention the way WhatsApp carries one", async () => {
    const onSend = vi.fn().mockResolvedValue(true);
    render(<GroupComposer {...props} group={waGroup()} onSend={onSend} />);

    const box = screen.getByPlaceholderText(/Tulis pesan ke grup/);
    await userEvent.type(box, "@Pak Joko cek ODP{Enter}");

    expect(onSend).toHaveBeenCalledWith("@55511122233344 cek ODP", [joko.jid]);
  });
});
