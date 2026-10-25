import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useChangePassword } from '@/hooks/use-system';
import { getToken, clearToken, setToken, isTokenRemembered } from '@/lib/api-client';
import { changePassword } from '@/api/endpoints';

vi.mock('@/api/endpoints', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/endpoints')>()),
  changePassword: vi.fn(),
}));
const mockedChangePassword = vi.mocked(changePassword);

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('useChangePassword', () => {
  beforeEach(() => {
    setToken('old-token');
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    clearToken();
    vi.restoreAllMocks();
    mockedChangePassword.mockReset();
  });

  // The server revokes every session on a password change and issues a fresh
  // token in the response. A client that keeps the old one is logged out on the
  // very next request, right after a "successful" password change.
  it('stores the rotated token returned by the server', async () => {
    mockedChangePassword.mockResolvedValue({
      status: 'ok',
      token: 'rotated-token',
      expires_in: 86400,
      revoked_sessions: 3,
    });

    const { result } = renderHook(() => useChangePassword(), { wrapper });
    result.current.mutate({ current_password: 'old-password', new_password: 'new-password' });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockedChangePassword).toHaveBeenCalledWith({
      current_password: 'old-password',
      new_password: 'new-password',
    });
    expect(getToken()).toBe('rotated-token');
  });

  // A "don't remember me" session lives in sessionStorage. Replacing the token
  // must not promote it to localStorage, where it would outlive the tab.
  it('keeps a non-remembered session in sessionStorage', async () => {
    setToken('old-token', false);
    expect(isTokenRemembered()).toBe(false);

    mockedChangePassword.mockResolvedValue({
      status: 'ok',
      token: 'rotated-token',
      expires_in: 86400,
      revoked_sessions: 1,
    });

    const { result } = renderHook(() => useChangePassword(), { wrapper });
    result.current.mutate({ current_password: 'old-password', new_password: 'new-password' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getToken()).toBe('rotated-token');
    expect(isTokenRemembered()).toBe(false);
    expect(sessionStorage.getItem('openwrt-auth-token')).toBe('rotated-token');
    expect(localStorage.getItem('openwrt-auth-token')).toBeNull();
  });

  it('leaves the existing token untouched when the change fails', async () => {
    mockedChangePassword.mockRejectedValue(new Error('invalid current password'));

    const { result } = renderHook(() => useChangePassword(), { wrapper });
    result.current.mutate({ current_password: 'wrong', new_password: 'new-password' });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(getToken()).toBe('old-token');
  });
});
