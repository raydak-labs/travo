import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { WifiRepeaterSameRadioBanner } from '../wifi-repeater-same-radio-banner';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';

function renderBanner() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <ThemeProvider>
      <QueryClientProvider client={client}>
        <WifiRepeaterSameRadioBanner />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('WifiRepeaterSameRadioBanner', () => {
  it('renders fix button when health reports same-radio repeater conflict', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'warning',
          issues: ['Repeater: same radio'],
          repeater_same_radio_ap_sta: true,
        }),
      ),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /fix radio layout/i })).toBeInTheDocument();
  });

  it('confirms before reconciling, because it restarts wireless', async () => {
    const user = userEvent.setup();
    let posted = false;
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'warning',
          issues: [],
          repeater_same_radio_ap_sta: true,
        }),
      ),
      http.post(API_ROUTES.wifi.repeaterReconcile, async () => {
        posted = true;
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /fix radio layout/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /fix radio layout/i }));

    // One click must not restart the wireless subsystem.
    expect(posted).toBe(false);
    expect(screen.getByText(/Move the uplink to the other radio/i)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /^apply$/i }));

    await waitFor(() => {
      expect(posted).toBe(true);
    });
  });
});
