import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '@/mocks/server';
import { getToken, setToken } from '@/lib/api-client';
import { useBackup, useRestore, useFirmwareUpgrade } from '../use-system';

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

const assignMock = vi.fn();

beforeEach(() => {
  setToken('test-token');
  vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:backup');
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
  // jsdom cannot navigate; the shared 401 handler's redirect is what we assert.
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: assignMock },
  });
});

afterEach(() => {
  assignMock.mockReset();
  vi.unstubAllGlobals();
  server.resetHandlers();
});

describe('useBackup', () => {
  it('downloads through the shared client', async () => {
    server.use(
      http.get('/api/v1/system/backup', () => new HttpResponse('tarball', { status: 200 })),
    );
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { result } = renderHook(() => useBackup(), { wrapper });
    await result.current.mutateAsync();

    expect(click).toHaveBeenCalled();
    expect(getToken()).toBe('test-token');
    click.mockRestore();
  });

  it('routes a 401 through the shared unauthorized handler', async () => {
    server.use(
      http.get('/api/v1/system/backup', () =>
        HttpResponse.json({ error: 'Unauthorized' }, { status: 401 }),
      ),
    );

    const { result } = renderHook(() => useBackup(), { wrapper });
    await expect(result.current.mutateAsync()).rejects.toThrow('Unauthorized');

    await waitFor(() => expect(getToken()).toBeNull());
    expect(assignMock).toHaveBeenCalledWith('/login');
  });
});

describe('useRestore / useFirmwareUpgrade', () => {
  it('surfaces an HTML error body without a SyntaxError', async () => {
    server.use(
      http.post('/api/v1/system/restore', () =>
        HttpResponse.text('<html>502 Bad Gateway</html>', { status: 502 }),
      ),
    );

    const { result } = renderHook(() => useRestore(), { wrapper });
    await expect(result.current.mutateAsync(new File(['x'], 'b.tar.gz'))).rejects.toThrow(
      'Request failed with status 502',
    );
  });

  it('reports the router error message for a failed upgrade', async () => {
    server.use(
      http.post('/api/v1/system/firmware/upgrade', () =>
        HttpResponse.json({ error: 'image too large' }, { status: 400 }),
      ),
    );

    const { result } = renderHook(() => useFirmwareUpgrade(), { wrapper });
    await expect(
      result.current.mutateAsync({ file: new File(['x'], 'fw.bin'), keepSettings: true }),
    ).rejects.toThrow('image too large');
  });

  it('clears the session when a restore is rejected as unauthorized', async () => {
    server.use(
      http.post('/api/v1/system/restore', () =>
        HttpResponse.json({ error: 'Unauthorized' }, { status: 401 }),
      ),
    );

    const { result } = renderHook(() => useRestore(), { wrapper });
    await expect(result.current.mutateAsync(new File(['x'], 'b.tar.gz'))).rejects.toThrow();

    await waitFor(() => expect(getToken()).toBeNull());
  });
});
