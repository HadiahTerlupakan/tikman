import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { UserRole, type User } from "@/domain/entities";
import {
  PICKER_ROWS,
  choosePageSize,
  times,
  visibleRows,
} from "@/test/pagination";
import { UserTable } from "./UserTable";

describe("UserTable", () => {
  it("lets the page-size picker change how many users show", async () => {
    const users: User[] = times(PICKER_ROWS, (i) => ({
      id: `user-${i}`,
      username: `user${i}`,
      email: `user${i}@example.com`,
      initials: "US",
      role: UserRole.TECHNICIAN,
      createdAt: "2026-10-13T12:00:00Z",
      updatedAt: "2026-10-13T12:00:00Z",
    }));
    render(
      <UserTable
        users={users}
        loading={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(visibleRows()).toBe(10);

    await choosePageSize(100);

    expect(visibleRows()).toBe(PICKER_ROWS);
  });
});
