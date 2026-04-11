import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { useSetAPConfig } from '@/hooks/use-wifi';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';
import * as routerRefresh from '@/lib/router-state-refresh';

beforeEach(() => {
  localStorage.setItem('openwrt-auth-token', 'test-token');
});

describe('useSetAPConfig', () => {
  it('refreshes wifi health and connection after successful save', async () => {
    const refreshSpy = vi.spyOn(routerRefresh, 'refreshRouterState').mockResolvedValue();
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    server.use(
      http.put(`${API_ROUTES.wifi.ap}/:section`, () => HttpResponse.json({ status: 'ok' })),
    );

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useSetAPConfig(), { wrapper });

    await result.current.mutateAsync({
      section: 'default_radio0',
      config: { ssid: 'Travel', encryption: 'psk2', key: 'password1' },
    });

    await waitFor(() => {
      expect(refreshSpy).toHaveBeenCalledWith(qc, [
        ['wifi', 'health'],
        ['wifi', 'connection'],
      ]);
    });

    refreshSpy.mockRestore();
  });
});
