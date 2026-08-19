import "./setupMocks";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { UserRole } from "@/domain/entities/User";
import { CreateUserDto, UpdateUserDto } from "@/domain/entities/User";
import {
  useCreateUser,
  useUpdateUser,
  useDeleteUser,
} from "@/application/hooks/useUsers";
import * as repositories from "@/infrastructure/repositories";
import { createWrapper } from "./setupMocks";

// Get mock objects
const { mockUserRepo } = (
  repositories as unknown as {
    __mocks: {
      mockUserRepo: {
        create: ReturnType<typeof vi.fn>;
        update: ReturnType<typeof vi.fn>;
        delete: ReturnType<typeof vi.fn>;
        getAll: ReturnType<typeof vi.fn>;
        getById: ReturnType<typeof vi.fn>;
      };
    };
  }
).__mocks;

describe("User Hooks - Query Invalidation", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    vi.clearAllMocks();
  });

  it("should invalidate users queries on useCreateUser", async () => {
    mockUserRepo.create.mockResolvedValue({ id: "1", username: "user1" });

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateUser(), { wrapper });

    const newUser: CreateUserDto = {
      username: "user1",
      email: "user1@example.com",
      password: "pass",
      role: UserRole.VIEWER,
    };

    act(() => {
      result.current.mutate(newUser);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["users"] });
  });
});

describe("User Hooks - Repository Integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should call UserRepository.update with correct parameters", async () => {
    mockUserRepo.update.mockResolvedValue({ id: "1", username: "updated" });

    const { result } = renderHook(() => useUpdateUser(), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate({
        id: "1",
        data: { username: "updated" } as UpdateUserDto,
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockUserRepo.update).toHaveBeenCalledWith("1", {
      username: "updated",
    });
  });

  it("should call UserRepository.delete with correct id", async () => {
    mockUserRepo.delete.mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteUser(), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate("1");
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockUserRepo.delete).toHaveBeenCalledWith("1");
  });
});
