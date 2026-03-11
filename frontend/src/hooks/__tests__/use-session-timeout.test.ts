import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { getTokenExpiry, useSessionTimeout } from '../use-session-timeout';

// Mock api-client
vi.mock('@/lib/api-client', () => ({
  getToken: vi.fn(),
  handleUnauthorized: vi.fn(),
}));

// Mock sonner
vi.mock('sonner', () => ({
  toast: { warning: vi.fn() },
}));

import { getToken, handleUnauthorized } from '@/lib/api-client';
import { toast } from 'sonner';

const mockedGetToken = vi.mocked(getToken);
const mockedHandleUnauthorized = vi.mocked(handleUnauthorized);
const mockedToastWarning = vi.mocked(toast.warning);

/** Build a fake JWT with a given exp claim. */
function fakeJwt(exp: number): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = btoa(JSON.stringify({ sub: 'admin', exp }));
  return `${header}.${payload}.fakesignature`;
}

describe('getTokenExpiry', () => {
  it('returns exp from a valid JWT', () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    expect(getTokenExpiry(fakeJwt(exp))).toBe(exp);
  });

  it('returns null for invalid token', () => {
    expect(getTokenExpiry('not-a-jwt')).toBeNull();
  });

  it('returns null for token without exp', () => {
    const header = btoa(JSON.stringify({ alg: 'HS256' }));
    const payload = btoa(JSON.stringify({ sub: 'admin' }));
    expect(getTokenExpiry(`${header}.${payload}.sig`)).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(getTokenExpiry('')).toBeNull();
  });
});

describe('useSessionTimeout', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does nothing when no token is present', () => {
    mockedGetToken.mockReturnValue(null);
    renderHook(() => useSessionTimeout());

    expect(mockedToastWarning).not.toHaveBeenCalled();
    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('does nothing when token has plenty of time left', () => {
    const exp = Math.floor(Date.now() / 1000) + 3600; // 1 hour
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    renderHook(() => useSessionTimeout());

    expect(mockedToastWarning).not.toHaveBeenCalled();
    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('shows warning when less than 5 minutes remain', () => {
    const exp = Math.floor(Date.now() / 1000) + 180; // 3 minutes
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    renderHook(() => useSessionTimeout());

    expect(mockedToastWarning).toHaveBeenCalledWith('Session expiring soon', {
      description: expect.stringContaining('3 minute'),
      duration: 10_000,
    });
  });

  it('shows warning only once', () => {
    const exp = Math.floor(Date.now() / 1000) + 180;
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    renderHook(() => useSessionTimeout());

    expect(mockedToastWarning).toHaveBeenCalledTimes(1);

    // Advance past next check interval
    act(() => {
      vi.advanceTimersByTime(30_000);
    });

    expect(mockedToastWarning).toHaveBeenCalledTimes(1);
  });

  it('calls handleUnauthorized when token is expired', () => {
    const exp = Math.floor(Date.now() / 1000) - 10; // already expired
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    renderHook(() => useSessionTimeout());

    expect(mockedHandleUnauthorized).toHaveBeenCalled();
  });

  it('detects expiry on interval tick', () => {
    // Start with 40 seconds left — inside warning threshold but not expired
    const nowSec = Math.floor(Date.now() / 1000);
    const exp = nowSec + 40;
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    renderHook(() => useSessionTimeout());

    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();

    // Advance past two interval ticks (30s each = 60s) — token is now expired
    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    expect(mockedHandleUnauthorized).toHaveBeenCalled();
  });

  it('cleans up interval on unmount', () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    mockedGetToken.mockReturnValue(fakeJwt(exp));
    const { unmount } = renderHook(() => useSessionTimeout());

    unmount();

    // Advancing timers should not cause further calls
    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    expect(mockedToastWarning).not.toHaveBeenCalled();
  });
});
