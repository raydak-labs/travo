import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { useSetRadioRole } from '@/hooks/use-wifi';
import { toast } from 'sonner';

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  localStorage.setItem('openwrt-auth-token', 'test-token');
  vi.mocked(toast.success).mockClear();
});

describe('useSetRadioRole', () => {
  // A radio turned into an AP with no key gets a passphrase invented by the
  // service; the response is the only place it is ever returned.
  it('shows the generated passphrase when the service invents one', async () => {
    server.use(
      http.put(`${API_ROUTES.wifi.radios}/:name/role`, () =>
        HttpResponse.json({
          status: 'ok',
          generated_key: 'inv3nted-passphrase',
        }),
      ),
    );

    const { result } = renderHook(() => useSetRadioRole(), { wrapper });
    await result.current.mutateAsync({ name: 'radio1', role: 'ap' });

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(
        'Radio radio1 updated',
        expect.objectContaining({
          description: expect.stringContaining('inv3nted-passphrase'),
        }),
      );
    });
  });

  it('keeps the plain confirmation when no key was generated', async () => {
    server.use(
      http.put(`${API_ROUTES.wifi.radios}/:name/role`, () => HttpResponse.json({ status: 'ok' })),
    );

    const { result } = renderHook(() => useSetRadioRole(), { wrapper });
    await result.current.mutateAsync({ name: 'radio1', role: 'sta' });

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Radio role updated');
    });
  });
});
