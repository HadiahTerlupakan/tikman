import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SwitchConfigTab } from "./SwitchConfigTab";

const backupNow = vi.fn();
const diffArgs = vi.fn();
const backups = [
  {
    id: "b2",
    takenAt: "2026-11-11T01:00:00Z",
    reason: "daily",
    rawSha256: "x",
    size: 20,
  },
  {
    id: "b1",
    takenAt: "2026-11-10T01:00:00Z",
    reason: "manual",
    rawSha256: "y",
    size: 18,
  },
];

vi.mock("@/application/hooks/useSwitches", () => ({
  useSwitchBackups: () => ({ data: backups, isLoading: false }),
  useSwitchBackup: (_id: string, backupId: string | null) => ({
    data: backupId ? { id: backupId, masked: `config ${backupId}` } : undefined,
  }),
  useSwitchBackupDiff: (
    _id: string,
    from: string | null,
    to: string | null,
  ) => {
    diffArgs(from, to);
    return { data: from && to ? "-vlan 20\n+vlan 30" : undefined };
  },
  useSwitchBackupNow: () => ({ mutate: backupNow, isPending: false }),
}));

describe("SwitchConfigTab", () => {
  beforeEach(() => {
    backupNow.mockReset();
    diffArgs.mockReset();
  });

  it("shows the masked text of the backup picked", async () => {
    render(<SwitchConfigTab switchId="s-1" canBackup />);
    await userEvent.click(screen.getAllByRole("checkbox")[1]);
    expect(screen.getByText("config b2")).toBeTruthy();
  });

  it("diffs two picked backups, older first", async () => {
    render(<SwitchConfigTab switchId="s-1" canBackup />);
    const boxes = screen.getAllByRole("checkbox");
    await userEvent.click(boxes[1]);
    await userEvent.click(boxes[2]);

    expect(diffArgs).toHaveBeenLastCalledWith("b1", "b2");
    expect(screen.getByText("+vlan 30")).toBeTruthy();
  });

  it("starts a backup for admin and technician", async () => {
    render(<SwitchConfigTab switchId="s-1" canBackup />);
    await userEvent.click(
      screen.getByRole("button", { name: "Backup sekarang" }),
    );
    expect(backupNow).toHaveBeenCalledWith("s-1", expect.anything());
  });

  it("hides the backup button from other roles", () => {
    render(<SwitchConfigTab switchId="s-1" canBackup={false} />);
    expect(
      screen.queryByRole("button", { name: "Backup sekarang" }),
    ).toBeNull();
  });
});
