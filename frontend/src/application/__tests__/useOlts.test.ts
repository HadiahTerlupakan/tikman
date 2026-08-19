import "./setupMocks";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { CreateOltDto, UpdateOltDto, OltProtocol } from "@/domain/entities/Olt";
import {
  useCreateOlt,
  useUpdateOlt,
  useDeleteOlt,
} from "@/application/hooks/useOlts";
import * as repositories from "@/infrastructure/repositories";
import { createWrapper } from "./setupMocks";

// Get mock objects
const { mockOltRepo } = (
  repositories as unknown as {
    __mocks: {
      mockOltRepo: {
        create: ReturnType<typeof vi.fn>;
        update: ReturnType<typeof vi.fn>;
        delete: ReturnType<typeof vi.fn>;
        getAll: ReturnType<typeof vi.fn>;
        getById: ReturnType<typeof vi.fn>;
        getBySite: ReturnType<typeof vi.fn>;
      };
    };
  }
).__mocks;

describe("OLT Hooks - Query Invalidation", () => {
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

  it("should invalidate olts and sites queries on useCreateOlt", async () => {
    mockOltRepo.create.mockResolvedValue({ id: "1", name: "OLT1" });

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateOlt(), { wrapper });

    const newOlt: CreateOltDto = {
      name: "OLT1",
      siteId: "site1",
      ipAddress: "192.168.1.1",
      preferredProtocol: OltProtocol.SSH,
      username: "admin",
      password: "password",
    };

    act(() => {
      result.current.mutate(newOlt);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["olts"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["sites"] });
  });

  it("should invalidate olts and sites queries on useUpdateOlt", async () => {
    mockOltRepo.update.mockResolvedValue({ id: "1", name: "OLT1-Updated" });

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useUpdateOlt(), { wrapper });

    act(() => {
      result.current.mutate({
        id: "1",
        data: { name: "OLT1-Updated" } as UpdateOltDto,
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["olts"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["olts", "1"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["sites"] });
  });

  it("should invalidate olts and sites queries on useDeleteOlt", async () => {
    mockOltRepo.delete.mockResolvedValue(undefined);

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDeleteOlt(), { wrapper });

    act(() => {
      result.current.mutate("1");
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["olts"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["sites"] });
  });
});

describe("OLT Hooks - Repository Integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should call OltRepository.update with correct parameters", async () => {
    mockOltRepo.update.mockResolvedValue({ id: "1", name: "Updated OLT" });

    const { result } = renderHook(() => useUpdateOlt(), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate({
        id: "1",
        data: { name: "Updated OLT" } as UpdateOltDto,
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockOltRepo.update).toHaveBeenCalledWith("1", {
      name: "Updated OLT",
    });
  });
});
