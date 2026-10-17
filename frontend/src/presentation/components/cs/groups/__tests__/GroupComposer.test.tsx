import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupComposer } from "../GroupComposer";
import { waGroup } from "./fixtures";

const props = {
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

    expect(screen.queryByPlaceholderText("Tulis pesan ke grup")).toBeNull();
    expect(screen.getByText(/Hanya admin grup/)).toBeInTheDocument();
  });

  it("sends what was typed and clears the box once it left", async () => {
    const onSend = vi.fn().mockResolvedValue(true);
    render(<GroupComposer {...props} group={waGroup()} onSend={onSend} />);

    const box = screen.getByPlaceholderText("Tulis pesan ke grup");
    await userEvent.type(box, "siap meluncur{Enter}");

    expect(onSend).toHaveBeenCalledWith("siap meluncur");
    expect(box).toHaveValue("");
  });
});
