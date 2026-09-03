import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UserRole } from "@/domain/entities";
import type { User } from "@/domain/entities";
import { TransferPicker } from "../TransferPicker";

function user(id: string, username: string, role: UserRole): User {
  return {
    id,
    username,
    email: `${username}@example.com`,
    initials: username.slice(0, 2).toUpperCase(),
    role,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

const users = [
  user("u1", "budi", UserRole.CS),
  user("u2", "sari", UserRole.TECHNICIAN),
  user("u3", "tamu", UserRole.VIEWER),
];

describe("TransferPicker", () => {
  // A viewer is 403'd on every route in the module, and the holder already
  // has the thread — offering either parks a customer with the wrong person.
  it("offers only the people who can actually take the thread", async () => {
    render(<TransferPicker users={users} holderId="u1" onTransfer={vi.fn()} />);

    await userEvent.click(screen.getByRole("combobox"));

    expect(await screen.findByTitle("sari")).toBeInTheDocument();
    expect(screen.queryByTitle("tamu")).toBeNull();
    expect(screen.queryByTitle("budi")).toBeNull();
  });

  it("hands the picked user id to the caller", async () => {
    const onTransfer = vi.fn();
    render(<TransferPicker users={users} onTransfer={onTransfer} />);

    await userEvent.click(screen.getByRole("combobox"));
    await userEvent.click(await screen.findByTitle("sari"));

    expect(onTransfer).toHaveBeenCalledWith("u2", expect.anything());
  });
});
