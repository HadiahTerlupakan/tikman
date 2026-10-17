import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupList } from "../GroupList";
import { waGroup } from "./fixtures";

describe("GroupList", () => {
  it("shows the last sender and message under the group name", () => {
    render(
      <GroupList
        groups={[
          waGroup({
            lastMessageKind: "text",
            lastMessagePreview: "ODP 3 mati",
            lastMessageSender: "Budi",
            lastMessageAt: "2026-10-17T07:05:00Z",
          }),
        ]}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("Budi: ODP 3 mati")).toBeInTheDocument();
  });

  it("marks a group our number has left", () => {
    render(
      <GroupList
        groups={[
          waGroup({ id: "a", name: "Lama", left: true }),
          waGroup({ id: "b", name: "Aktif" }),
        ]}
        onSelect={vi.fn()}
      />,
    );

    const left = screen.getByRole("listitem", { name: "Lama" });
    const active = screen.getByRole("listitem", { name: "Aktif" });
    expect(within(left).getByText("Keluar")).toBeInTheDocument();
    expect(within(active).queryByText("Keluar")).not.toBeInTheDocument();
  });

  it("names the number only when it is given one to name", () => {
    const { rerender } = render(
      <GroupList groups={[waGroup()]} onSelect={vi.fn()} />,
    );
    expect(screen.queryByText("CS Cariu")).not.toBeInTheDocument();

    rerender(
      <GroupList
        groups={[waGroup()]}
        accountLabels={{ acc1: "CS Cariu" }}
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByText("CS Cariu")).toBeInTheDocument();
  });

  it("opens the group that was clicked", async () => {
    const onSelect = vi.fn();
    render(<GroupList groups={[waGroup({ id: "g7" })]} onSelect={onSelect} />);

    await userEvent.click(screen.getByRole("listitem"));

    expect(onSelect).toHaveBeenCalledWith("g7");
  });
});
