import { describe, it, expect } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { mockWifiConnection } from '@/mocks/data';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createRouter,
  createRoute,
  createRootRoute,
  RouterProvider,
  Outlet,
  createMemoryHistory,
} from '@tanstack/react-router';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { WifiPage } from '../wifi-page';

function renderWifiPage(initialPath = '/wifi') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const wifiAdvancedRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/wifi/advanced',
    component: WifiPage,
  });
  const wifiRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/wifi',
    component: WifiPage,
  });

  const routeTree = rootRoute.addChildren([wifiAdvancedRoute, wifiRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('WifiPage', () => {
  it('has no in-page tablist mirroring sidebar', async () => {
    renderWifiPage('/wifi');

    await waitFor(() => {
      expect(screen.getByText('Current connection')).toBeInTheDocument();
    });
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('tab', { name: /Wireless|Connect|Extras|Advanced/i }),
    ).not.toBeInTheDocument();
  });

  it('shows Connect panel content on /wifi', async () => {
    renderWifiPage('/wifi');

    await waitFor(() => {
      expect(screen.getByText('Current connection')).toBeInTheDocument();
    });
    expect(screen.queryByText('MAC Address Cloning')).not.toBeInTheDocument();
  });

  it('shows Advanced panel content on /wifi/advanced', async () => {
    renderWifiPage('/wifi/advanced');

    await waitFor(() => {
      // Every advanced entry is the same collapsible section, so the title is
      // the section trigger rather than a card heading.
      expect(screen.getByRole('button', { name: 'MAC address cloning' })).toBeInTheDocument();
    });
    expect(screen.queryByText('Current connection')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Radio hardware/i })).toBeInTheDocument();
  });

  it('renders current connection section with SSID', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Current connection')).toBeInTheDocument();
    });

    await waitFor(() => {
      const matches = screen.getAllByText('Hotel_Guest_5G');
      expect(matches.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders connected badge for active connection', async () => {
    renderWifiPage();

    await waitFor(() => {
      const matches = screen.getAllByText('Connected');
      expect(matches.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders saved networks section', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Saved networks')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Home_WiFi')).toBeInTheDocument();
      expect(screen.getByText('Office_Network')).toBeInTheDocument();
    });
  });

  it('renders scan networks button', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Scan')).toBeInTheDocument();
    });
  });

  it('joins a new uplink directly from Access Point mode', async () => {
    let joined = false;
    const applyToken = 'new-uplink';
    server.use(
      http.get(API_ROUTES.wifi.connection, () =>
        HttpResponse.json(
          joined
            ? { ...mockWifiConnection, mode: 'repeater', ssid: 'Airport_Free_WiFi' }
            : { mode: 'ap', connected: false },
        ),
      ),
      http.post(API_ROUTES.wifi.connect, async ({ request }) => {
        const body = await request.json();
        if (
          !body ||
          typeof body !== 'object' ||
          !('ssid' in body) ||
          body.ssid !== 'Airport_Free_WiFi'
        ) {
          return HttpResponse.json({ error: 'Unexpected uplink' }, { status: 400 });
        }
        return HttpResponse.json({
          status: 'ok',
          apply: { pending: true, token: applyToken, rollback_timeout_seconds: 30 },
        });
      }),
      http.post(API_ROUTES.wifi.applyConfirm, async ({ request }) => {
        expect(await request.json()).toEqual({ token: applyToken });
        joined = true;
        return HttpResponse.json({ status: 'ok' });
      }),
    );
    const user = userEvent.setup();
    renderWifiPage();

    await screen.findByRole('button', { name: /Access Point/ });
    await user.click(await screen.findByRole('button', { name: 'Scan networks' }));
    const dialog = screen.getByRole('dialog');
    const network = await within(dialog).findByText('Airport_Free_WiFi');
    const networkRow = within(dialog)
      .getAllByRole('listitem')
      .find((row) => row.contains(network));
    expect(networkRow).toBeDefined();
    await user.click(within(networkRow!).getByRole('button', { name: 'Connect' }));
    await user.click(within(dialog).getByRole('button', { name: 'Connect' }));

    expect(
      await within(await screen.findByRole('region', { name: 'Current connection' })).findByText(
        'Airport_Free_WiFi',
      ),
    ).toBeInTheDocument();
  });

  it('shows scan failure and lets the user retry', async () => {
    server.use(
      http.get(API_ROUTES.wifi.scan, () =>
        HttpResponse.json({ error: 'Radio unavailable' }, { status: 500 }),
      ),
    );
    const user = userEvent.setup();
    renderWifiPage();

    await user.click(await screen.findByRole('button', { name: 'Scan networks' }));
    const dialog = within(screen.getByRole('dialog'));
    expect(await dialog.findByText(/Wi-Fi scan failed/)).toBeInTheDocument();
    expect(dialog.queryByText('No networks found')).not.toBeInTheDocument();

    server.resetHandlers();
    await user.click(dialog.getByRole('button', { name: 'Scan' }));
    expect(await dialog.findByText('Airport_Free_WiFi')).toBeInTheDocument();
  });

  it('renders signal strength and IP for current connection', async () => {
    renderWifiPage();

    await waitFor(() => {
      // The summary band shows the same reading as the connection card, so both
      // matches are expected here.
      expect(screen.getAllByText(/82%/).length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText(/192\.168\.1\.105/)).toBeInTheDocument();
    });
  });
});
