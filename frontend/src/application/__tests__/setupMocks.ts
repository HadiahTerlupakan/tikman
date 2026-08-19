import { vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { User, UserRole } from "@/domain/entities/User";

// Mock repositories - must be defined before vi.mock for hoisting
vi.mock("@/infrastructure/repositories", () => {
  const mockAuthRepo = {
    login: vi.fn(),
    logout: vi.fn(),
    getCurrentUser: vi.fn(),
  };

  const mockUserRepo = {
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    getAll: vi.fn(),
    getById: vi.fn(),
  };

  const mockSiteRepo = {
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    getAll: vi.fn(),
    getById: vi.fn(),
  };

  const mockOltRepo = {
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    getAll: vi.fn(),
    getById: vi.fn(),
    getBySite: vi.fn(),
  };

  return {
    AuthRepository: class {
      login = mockAuthRepo.login;
      logout = mockAuthRepo.logout;
      getCurrentUser = mockAuthRepo.getCurrentUser;
    },
    UserRepository: class {
      create = mockUserRepo.create;
      update = mockUserRepo.update;
      delete = mockUserRepo.delete;
      getAll = mockUserRepo.getAll;
      getById = mockUserRepo.getById;
    },
    SiteRepository: class {
      create = mockSiteRepo.create;
      update = mockSiteRepo.update;
      delete = mockSiteRepo.delete;
      getAll = mockSiteRepo.getAll;
      getById = mockSiteRepo.getById;
    },
    OltRepository: class {
      create = mockOltRepo.create;
      update = mockOltRepo.update;
      delete = mockOltRepo.delete;
      getAll = mockOltRepo.getAll;
      getById = mockOltRepo.getById;
      getBySite = mockOltRepo.getBySite;
    },
    __mocks: {
      mockAuthRepo,
      mockUserRepo,
      mockSiteRepo,
      mockOltRepo,
    },
  };
});

export const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
};

export const createMockUser = (): User => ({
  id: "1",
  username: "admin",
  email: "admin@example.com",
  role: UserRole.ADMIN,
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
});
