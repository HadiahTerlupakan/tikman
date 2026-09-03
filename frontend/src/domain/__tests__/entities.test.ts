import { describe, it, expect } from "vitest";
import { UserRole, type User } from "../entities/User";

describe("User Entity", () => {
  it("should have correct UserRole enum values", () => {
    expect(UserRole.ADMIN).toBe("admin");
    expect(UserRole.TECHNICIAN).toBe("technician");
    expect(UserRole.VIEWER).toBe("viewer");
  });

  it("should create valid User object", () => {
    const user: User = {
      id: "123",
      username: "admin",
      email: "admin@test.com",
      initials: "AD",
      role: UserRole.ADMIN,
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    };

    expect(user.username).toBe("admin");
    expect(user.role).toBe(UserRole.ADMIN);
  });
});
