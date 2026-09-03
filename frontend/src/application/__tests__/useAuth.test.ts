import "./setupMocks";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useLogin, useLogout } from "@/application/hooks/useAuth";
import { useAuthStore } from "@/application/stores/authStore";
import * as repositories from "@/infrastructure/repositories";
import { refreshTokenIfGranted } from "@/infrastructure/firebase/messaging";
import { createWrapper, createMockUser } from "./setupMocks";

vi.mock("@/infrastructure/firebase/messaging", () => ({
  refreshTokenIfGranted: vi.fn(),
}));

const mockRefreshToken = vi.mocked(refreshTokenIfGranted);

// Get mock objects
const { mockAuthRepo, mockPushRepo } = (
  repositories as unknown as {
    __mocks: {
      mockAuthRepo: {
        login: ReturnType<typeof vi.fn>;
        logout: ReturnType<typeof vi.fn>;
        getCurrentUser: ReturnType<typeof vi.fn>;
      };
      mockPushRepo: {
        subscribe: ReturnType<typeof vi.fn>;
        unsubscribe: ReturnType<typeof vi.fn>;
      };
    };
  }
).__mocks;

describe("Auth Hooks", () => {
  beforeEach(() => {
    // Reset auth store before each test
    const { result } = renderHook(() => useAuthStore());
    act(() => {
      result.current.logout();
    });

    // Reset all mocks
    vi.clearAllMocks();
  });

  describe("useLogin", () => {
    it("should call login mutation and update auth store", async () => {
      const mockUser = { id: "1", username: "admin", role: "admin" };
      const mockResponse = { user: mockUser, token: "test-token" };
      mockAuthRepo.login.mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useLogin(), {
        wrapper: createWrapper(),
      });
      const { result: authResult } = renderHook(() => useAuthStore());

      expect(result.current.isPending).toBe(false);

      act(() => {
        result.current.mutate({ username: "admin", password: "password" });
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockAuthRepo.login).toHaveBeenCalledWith({
        username: "admin",
        password: "password",
      });
      expect(authResult.current.user).toEqual(mockUser);
      expect(authResult.current.isAuthenticated).toBe(true);
    });
  });

  describe("useLogout", () => {
    it("should call logout mutation and clear auth store", async () => {
      mockAuthRepo.logout.mockResolvedValue(undefined);

      // Set a user first
      const { result: authResult } = renderHook(() => useAuthStore());
      act(() => {
        authResult.current.setUser(createMockUser());
      });

      expect(authResult.current.isAuthenticated).toBe(true);

      const { result } = renderHook(() => useLogout(), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockAuthRepo.logout).toHaveBeenCalled();
      expect(authResult.current.user).toBeNull();
      expect(authResult.current.isAuthenticated).toBe(false);
    });

    // A notification body carries the customer's name and their words, so a
    // device that keeps its registration after logout shows another team's
    // inbox to whoever is holding the phone.
    it("drops this device's push registration on the way out", async () => {
      mockAuthRepo.logout.mockResolvedValue(undefined);
      mockRefreshToken.mockResolvedValue("device-token");
      mockPushRepo.unsubscribe.mockResolvedValue(undefined);

      const { result } = renderHook(() => useLogout(), {
        wrapper: createWrapper(),
      });
      act(() => {
        result.current.mutate();
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockPushRepo.unsubscribe).toHaveBeenCalledWith("device-token");
    });

    it("still logs out when the unsubscribe call fails", async () => {
      mockAuthRepo.logout.mockResolvedValue(undefined);
      mockRefreshToken.mockResolvedValue("device-token");
      mockPushRepo.unsubscribe.mockRejectedValue(new Error("network down"));

      const { result: authResult } = renderHook(() => useAuthStore());
      act(() => {
        authResult.current.setUser(createMockUser());
      });

      const { result } = renderHook(() => useLogout(), {
        wrapper: createWrapper(),
      });
      act(() => {
        result.current.mutate();
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockAuthRepo.logout).toHaveBeenCalled();
      expect(authResult.current.isAuthenticated).toBe(false);
    });
  });
});
