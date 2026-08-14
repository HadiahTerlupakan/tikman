import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';

// Mock repositories - must be defined before vi.mock for hoisting
vi.mock('@/infrastructure/repositories', () => {
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

// Import hooks after mocking
import { useLogin, useLogout } from '../hooks/useAuth';
import { useCreateUser, useUpdateUser, useDeleteUser } from '../hooks/useUsers';
import { useCreateSite, useUpdateSite, useDeleteSite } from '../hooks/useSites';
import { useCreateOlt, useUpdateOlt, useDeleteOlt } from '../hooks/useOlts';
import { useAuthStore } from '../stores/authStore';
import * as repositories from '@/infrastructure/repositories';

// Get mock objects
const { mockAuthRepo, mockUserRepo, mockSiteRepo, mockOltRepo } = (repositories as any).__mocks;

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
};

describe('Auth Hooks', () => {
  beforeEach(() => {
    // Reset auth store before each test
    const { result } = renderHook(() => useAuthStore());
    act(() => {
      result.current.logout();
    });

    // Reset all mocks
    vi.clearAllMocks();
  });

  describe('useLogin', () => {
    it('should call login mutation and update auth store', async () => {
      const mockUser = { id: '1', username: 'admin', role: 'admin' };
      const mockResponse = { user: mockUser, token: 'test-token' };
      mockAuthRepo.login.mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useLogin(), { wrapper: createWrapper() });
      const { result: authResult } = renderHook(() => useAuthStore());

      expect(result.current.isPending).toBe(false);

      act(() => {
        result.current.mutate({ username: 'admin', password: 'password' });
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockAuthRepo.login).toHaveBeenCalledWith({ username: 'admin', password: 'password' });
      expect(authResult.current.user).toEqual(mockUser);
      expect(authResult.current.isAuthenticated).toBe(true);
    });
  });

  describe('useLogout', () => {
    it('should call logout mutation and clear auth store', async () => {
      mockAuthRepo.logout.mockResolvedValue(undefined);

      // Set a user first
      const { result: authResult } = renderHook(() => useAuthStore());
      act(() => {
        authResult.current.setUser({ id: '1', username: 'admin', role: 'admin' } as any);
      });

      expect(authResult.current.isAuthenticated).toBe(true);

      const { result } = renderHook(() => useLogout(), { wrapper: createWrapper() });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(mockAuthRepo.logout).toHaveBeenCalled();
      expect(authResult.current.user).toBeNull();
      expect(authResult.current.isAuthenticated).toBe(false);
    });
  });
});

describe('Query Invalidation', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    vi.clearAllMocks();
  });

  it('should invalidate olts and sites queries on useCreateOlt', async () => {
    mockOltRepo.create.mockResolvedValue({ id: '1', name: 'OLT1' });

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateOlt(), { wrapper });

    act(() => {
      result.current.mutate({ name: 'OLT1', siteId: 'site1' } as any);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['olts'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sites'] });
  });

  it('should invalidate olts and sites queries on useUpdateOlt', async () => {
    mockOltRepo.update.mockResolvedValue({ id: '1', name: 'OLT1-Updated' });

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useUpdateOlt(), { wrapper });

    act(() => {
      result.current.mutate({ id: '1', data: { name: 'OLT1-Updated' } as any });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['olts'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['olts', '1'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sites'] });
  });

  it('should invalidate olts and sites queries on useDeleteOlt', async () => {
    mockOltRepo.delete.mockResolvedValue(undefined);

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDeleteOlt(), { wrapper });

    act(() => {
      result.current.mutate('1');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['olts'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sites'] });
  });

  it('should invalidate users queries on useCreateUser', async () => {
    mockUserRepo.create.mockResolvedValue({ id: '1', username: 'user1' });

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateUser(), { wrapper });

    act(() => {
      result.current.mutate({ username: 'user1', password: 'pass' } as any);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['users'] });
  });

  it('should invalidate sites queries on useCreateSite', async () => {
    mockSiteRepo.create.mockResolvedValue({ id: '1', name: 'Site1' });

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useCreateSite(), { wrapper });

    act(() => {
      result.current.mutate({ name: 'Site1' } as any);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sites'] });
  });
});

describe('Repository Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should call UserRepository.update with correct parameters', async () => {
    mockUserRepo.update.mockResolvedValue({ id: '1', username: 'updated' });

    const { result } = renderHook(() => useUpdateUser(), { wrapper: createWrapper() });

    act(() => {
      result.current.mutate({ id: '1', data: { username: 'updated' } as any });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockUserRepo.update).toHaveBeenCalledWith('1', { username: 'updated' });
  });

  it('should call SiteRepository.update with correct parameters', async () => {
    mockSiteRepo.update.mockResolvedValue({ id: '1', name: 'Updated Site' });

    const { result } = renderHook(() => useUpdateSite(), { wrapper: createWrapper() });

    act(() => {
      result.current.mutate({ id: '1', data: { name: 'Updated Site' } as any });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockSiteRepo.update).toHaveBeenCalledWith('1', { name: 'Updated Site' });
  });

  it('should call OltRepository.update with correct parameters', async () => {
    mockOltRepo.update.mockResolvedValue({ id: '1', name: 'Updated OLT' });

    const { result } = renderHook(() => useUpdateOlt(), { wrapper: createWrapper() });

    act(() => {
      result.current.mutate({ id: '1', data: { name: 'Updated OLT' } as any });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockOltRepo.update).toHaveBeenCalledWith('1', { name: 'Updated OLT' });
  });

  it('should call UserRepository.delete with correct id', async () => {
    mockUserRepo.delete.mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteUser(), { wrapper: createWrapper() });

    act(() => {
      result.current.mutate('1');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockUserRepo.delete).toHaveBeenCalledWith('1');
  });

  it('should call SiteRepository.delete with correct id', async () => {
    mockSiteRepo.delete.mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteSite(), { wrapper: createWrapper() });

    act(() => {
      result.current.mutate('1');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockSiteRepo.delete).toHaveBeenCalledWith('1');
  });
});
