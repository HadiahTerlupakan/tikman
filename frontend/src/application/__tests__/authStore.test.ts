import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAuthStore } from "@/application/stores/authStore";
import { User, UserRole } from "@/domain/entities/User";

describe("Auth Store", () => {
  beforeEach(() => {
    const { result } = renderHook(() => useAuthStore());
    act(() => {
      result.current.logout();
    });
  });

  it("should initialize with unauthenticated state", () => {
    const { result } = renderHook(() => useAuthStore());

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });

  it("should set user on successful login", async () => {
    const { result } = renderHook(() => useAuthStore());
    const mockUser: User = {
      id: "1",
      username: "admin",
      email: "admin@example.com",
      initials: "AD",
      role: UserRole.ADMIN,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    act(() => {
      result.current.setUser(mockUser);
    });

    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.user).toEqual(mockUser);
  });

  it("should clear user on logout", async () => {
    const { result } = renderHook(() => useAuthStore());
    const mockUser: User = {
      id: "1",
      username: "admin",
      email: "admin@example.com",
      initials: "AD",
      role: UserRole.ADMIN,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    act(() => {
      result.current.setUser(mockUser);
    });
    expect(result.current.isAuthenticated).toBe(true);

    act(() => {
      result.current.logout();
    });

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });
});
