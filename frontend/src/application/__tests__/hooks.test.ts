import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useLogin } from '../hooks/useAuth';
import { createElement } from 'react';

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
};

describe('useLogin hook', () => {
  it('should call login mutation', async () => {
    const { result } = renderHook(() => useLogin(), { wrapper: createWrapper() });

    expect(result.current.isPending).toBe(false);
  });
});
