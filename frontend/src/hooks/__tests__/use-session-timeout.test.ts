import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSessionTimeout } from '../use-session-timeout';

// Mock api-client
vi.mock('@/lib/api-client', () => ({
  apiClient: { get: vi.fn() },
  getToken: vi.fn(),
  handleUnauthorized: vi.fn(),
}));

// Mock sonner
vi.mock('sonner', () => ({
  toast: { warning: vi.fn() },
}));

import { apiClient, getToken, handleUnauthorized } from '@/lib/api-client';
import { toast } from 'sonner';

const mockedGet = vi.mocked(apiClient.get);
const mockedGetToken = vi.mocked(getToken);
const mockedHandleUnauthorized = vi.mocked(handleUnauthorized);
const mockedToastWarning = vi.mocked(toast.warning);

function mockSession(expiresIn: number) {
  mockedGet.mockResolvedValue({ valid: true, expires_in: expiresIn });
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('useSessionTimeout', () => {
  beforeEach(() => {
    vi.useFakeTimers({
      toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout', 'Date', 'performance'],
    });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not query the session when no token is present', async () => {
    mockedGetToken.mockReturnValue(null);
    renderHook(() => useSessionTimeout());
    await flush();

    expect(mockedGet).not.toHaveBeenCalled();
    expect(mockedToastWarning).not.toHaveBeenCalled();
    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('fetches remaining lifetime from the server and stays quiet with time left', async () => {
    mockedGetToken.mockReturnValue('token');
    mockSession(3600);
    renderHook(() => useSessionTimeout());
    await flush();

    expect(mockedGet).toHaveBeenCalledWith('/api/v1/auth/session');
    expect(mockedToastWarning).not.toHaveBeenCalled();
    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('warns when less than 5 minutes remain', async () => {
    mockedGetToken.mockReturnValue('token');
    mockSession(180); // 3 minutes
    renderHook(() => useSessionTimeout());
    await flush();

    act(() => {
      vi.advanceTimersByTime(30_000);
    });

    expect(mockedToastWarning).toHaveBeenCalledWith('Session expiring soon', {
      description: expect.stringContaining('minute'),
      duration: 10_000,
    });
  });

  it('warns only once', async () => {
    mockedGetToken.mockReturnValue('token');
    mockSession(280);
    renderHook(() => useSessionTimeout());
    await flush();

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(mockedToastWarning).toHaveBeenCalledTimes(1);

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(mockedToastWarning).toHaveBeenCalledTimes(1);
  });

  it('re-validates with the server when the local countdown runs out', async () => {
    mockedGetToken.mockReturnValue('token');
    mockSession(40);
    renderHook(() => useSessionTimeout());
    await flush();
    expect(mockedGet).toHaveBeenCalledTimes(1);

    // Past the deadline: hook must ask the server again instead of logging
    // out based on local clock math.
    await act(async () => {
      vi.advanceTimersByTime(60_000);
      await Promise.resolve();
    });

    expect(mockedGet.mock.calls.length).toBeGreaterThan(1);
    // The server said the session is still valid — no local logout.
    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('does not log out locally when the session check fails (e.g. offline)', async () => {
    mockedGetToken.mockReturnValue('token');
    mockedGet.mockRejectedValue(new Error('network down'));
    renderHook(() => useSessionTimeout());
    await flush();

    act(() => {
      vi.advanceTimersByTime(120_000);
    });
    await flush();

    expect(mockedHandleUnauthorized).not.toHaveBeenCalled();
  });

  it('cleans up interval on unmount', async () => {
    mockedGetToken.mockReturnValue('token');
    mockSession(280);
    const { unmount } = renderHook(() => useSessionTimeout());
    await flush();

    unmount();

    act(() => {
      vi.advanceTimersByTime(120_000);
    });

    expect(mockedToastWarning).not.toHaveBeenCalled();
  });
});
