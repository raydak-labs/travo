import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { APStep } from '@/pages/setup/ap-step';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';
import { toast } from 'sonner';

function renderAPStep(onNext = () => {}, onBack = () => {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <APStep onNext={onNext} onBack={onBack} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('APStep', () => {
  beforeEach(() => {
    localStorage.setItem('openwrt-auth-token', 'test-token');
  });

  it('PUTs unified SSID and key to every AP section', async () => {
    const user = userEvent.setup();
    const putPaths: string[] = [];

    server.use(
      http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ request, params }) => {
        putPaths.push(params.section as string);
        const body = (await request.json()) as { ssid: string; key: string };
        expect(body.ssid).toBe('TravelSSID');
        expect(body.key).toBe('travel12345');
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    const onNext = vi.fn();
    renderAPStep(onNext);

    await waitFor(() => {
      expect(screen.getByLabelText(/network name \(ssid\)/i)).toBeInTheDocument();
    });

    const ssidInput = screen.getByLabelText(/network name \(ssid\)/i);
    const keyInput = screen.getByLabelText(/^password$/i);

    await user.clear(ssidInput);
    await user.type(ssidInput, 'TravelSSID');
    await user.clear(keyInput);
    await user.type(keyInput, 'travel12345');

    await user.click(screen.getByRole('button', { name: /save ap config/i }));

    await waitFor(() => {
      expect(onNext).toHaveBeenCalled();
    });

    expect(putPaths.sort()).toEqual(['default_radio0', 'default_radio1']);
  });

  // Setup writes both bands in sequence; without a rollback the router is left
  // with the new SSID on one band and the old one on the other.
  it('restores the first band when the second one fails', async () => {
    const user = userEvent.setup();
    const putBodies: Array<{ section: string; ssid: string }> = [];
    const errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '');

    server.use(
      http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ request, params }) => {
        const body = (await request.json()) as { ssid: string };
        putBodies.push({ section: params.section as string, ssid: body.ssid });
        if (body.ssid === 'TravelSSID' && params.section === 'default_radio1') {
          return HttpResponse.json({ error: 'uci commit failed' }, { status: 400 });
        }
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    const onNext = vi.fn();
    renderAPStep(onNext);

    await waitFor(() => {
      expect(screen.getByLabelText(/network name \(ssid\)/i)).toBeInTheDocument();
    });

    await user.clear(screen.getByLabelText(/network name \(ssid\)/i));
    await user.type(screen.getByLabelText(/network name \(ssid\)/i), 'TravelSSID');
    await user.clear(screen.getByLabelText(/^password$/i));
    await user.type(screen.getByLabelText(/^password$/i), 'travel12345');

    await user.click(screen.getByRole('button', { name: /save ap config/i }));

    await waitFor(() => {
      expect(putBodies.length).toBe(3);
    });
    expect(putBodies).toEqual([
      { section: 'default_radio0', ssid: 'TravelSSID' },
      { section: 'default_radio1', ssid: 'TravelSSID' },
      { section: 'default_radio0', ssid: 'OpenWrt-Travel' },
    ]);
    expect(onNext).not.toHaveBeenCalled();
    expect(errorSpy).toHaveBeenCalledWith(
      'Failed to update AP config',
      expect.objectContaining({
        description: expect.stringContaining('default_radio0'),
      }),
    );

    errorSpy.mockRestore();
  });
});
