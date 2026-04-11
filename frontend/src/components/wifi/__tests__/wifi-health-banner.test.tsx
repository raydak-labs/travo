import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { WifiHealthBanner } from '../wifi-health-banner';
import { http, HttpResponse } from 'msw';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';

function renderBanner() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <WifiHealthBanner />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('WifiHealthBanner', () => {
  it('renders nothing when status is ok', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'ok',
          issues: [],
          sta: { ifname: 'wlan0', ssid: 'Hotel-WiFi', associated: true },
          wwan: { device: 'wlan0', up: true, ip_address: '10.0.0.50' },
        }),
      ),
    );

    const { container } = renderBanner();

    // Wait for query to settle, then verify no alert rendered
    await waitFor(() => {
      expect(container.querySelector('[role="alert"]')).toBeNull();
    });
  });

  it('renders a red banner on error status with issue messages', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'error',
          issues: [
            'wwan network is bound to wlan1 but active STA runs on wlan0 — netifd cannot assign an IP to the connected radio',
          ],
          sta: { ifname: 'wlan0', ssid: 'Hotel-WiFi', associated: true },
          wwan: { device: 'wlan1', up: false, ip_address: '' },
        }),
      ),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText(/WiFi configuration mismatch/i)).toBeInTheDocument();
    expect(screen.getByText(/wwan network is bound to wlan1/i)).toBeInTheDocument();
  });

  it('renders nothing when the only warning is repeater same-radio (shown in dedicated banner)', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'warning',
          issues: [
            'Repeater: Wi‑Fi uplink (STA) and an access point are on the same radio — use the other radio for downlink AP, or enable “Wi‑Fi on uplink radio” in repeater options.',
          ],
          repeater_same_radio_ap_sta: true,
        }),
      ),
    );

    const { container } = renderBanner();

    await waitFor(() => {
      expect(container.querySelector('[role="alert"]')).toBeNull();
    });
  });

  it('after filtering repeater issue, shows DHCP warning title and no duplicate repeater bullet', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'warning',
          issues: [
            'STA is associated to "Hotel-WiFi" but wwan has no DHCP lease yet',
            'Repeater: Wi‑Fi uplink (STA) and an access point are on the same radio — use the other radio for downlink AP, or enable “Wi‑Fi on uplink radio” in repeater options.',
          ],
          repeater_same_radio_ap_sta: true,
        }),
      ),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText(/Waiting for IP address/i)).toBeInTheDocument();
    expect(screen.queryByText(/Repeater:/i)).not.toBeInTheDocument();
  });

  it('renders an amber banner on warning status', async () => {
    server.use(
      http.get(API_ROUTES.wifi.health, () =>
        HttpResponse.json({
          status: 'warning',
          issues: ['STA is associated with Hotel-WiFi but wwan has no IP lease yet'],
          sta: { ifname: 'wlan0', ssid: 'Hotel-WiFi', associated: true },
          wwan: { device: 'wlan0', up: false, ip_address: '' },
        }),
      ),
    );

    renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText(/Waiting for IP/i)).toBeInTheDocument();
  });
});
