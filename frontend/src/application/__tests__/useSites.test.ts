import "./setupMocks";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { CreateSiteDto, UpdateSiteDto } from "@/domain/entities/Site";
import {
  useCreateSite,
  useUpdateSite,
  useDeleteSite,
} from "@/application/hooks/useSites";
import * as repositories from "@/infrastructure/repositories";
import { createWrapper } from "./setupMocks";

// Get mock objects
const { mockSiteRepo } = (
  repositories as unknown as {
    __mocks: {
      mockSiteRepo: {
        create: ReturnType<typeof vi.fn>;
        update: ReturnType<typeof vi.fn>;
        delete: ReturnType<typeof vi.fn>;
        getAll: ReturnType<typeof vi.fn>;
        getById: ReturnType<typeof vi.fn>;
      };
    };
  }
).__mocks;

describe("Site Hooks - Query Invalidation", () => {
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

  it("should invalidate sites queries on useCreateSite", async () => {
    mockSiteRepo.create.mockResolvedValue({ id: "1", name: "Site1" });

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateSite(), { wrapper });

    const newSite: CreateSiteDto = { name: "Site1" };

    act(() => {
      result.current.mutate(newSite);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["sites"] });
  });
});

describe("Site Hooks - Repository Integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should call SiteRepository.update with correct parameters", async () => {
    mockSiteRepo.update.mockResolvedValue({ id: "1", name: "Updated Site" });

    const { result } = renderHook(() => useUpdateSite(), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate({
        id: "1",
        data: { name: "Updated Site" } as UpdateSiteDto,
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockSiteRepo.update).toHaveBeenCalledWith("1", {
      name: "Updated Site",
    });
  });

  it("should call SiteRepository.delete with correct id", async () => {
    mockSiteRepo.delete.mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteSite(), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate("1");
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockSiteRepo.delete).toHaveBeenCalledWith("1");
  });
});
