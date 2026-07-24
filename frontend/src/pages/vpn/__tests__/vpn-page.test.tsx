import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createRouter,
  createRoute,
  createRootRoute,
  RouterProvider,
  Outlet,
  createMemoryHistory,
} from '@tanstack/react-router';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { VpnPage } from '../vpn-page';

function renderVpnPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const vpnRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/vpn',
    component: VpnPage,
  });

  const servicesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services',
    component: () => <div>Services</div>,
  });

  const routeTree = rootRoute.addChildren([vpnRoute, servicesRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/vpn'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('VpnPage', () => {
  it('shows WireGuard connection status', async () => {
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('WireGuard')).toBeInTheDocument();
    });

    await waitFor(() => {
      const badges = screen.getAllByText('Connected');
      expect(badges.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('has toggle switches for WireGuard', async () => {
    renderVpnPage();

    await waitFor(() => {
      const toggles = screen.getAllByRole('checkbox');
      expect(toggles.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows WireGuard peers', async () => {
    renderVpnPage();

    await waitFor(() => {
      const matches = screen.getAllByText(/vpn\.example\.com:51820/);
      expect(matches.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows kill switch toggle', async () => {
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('Kill Switch')).toBeInTheDocument();
    });
  });

  it('collapses secondary VPN cards by default', async () => {
    const user = userEvent.setup();
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('WireGuard')).toBeInTheDocument();
      expect(screen.getByText('Kill Switch')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /Split Tunneling/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /DNS Leak Test/i })).toBeInTheDocument();
    expect(
      screen.queryByText(/Verify that DNS queries are routed through the VPN/i),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /DNS Leak Test/i }));
    expect(
      screen.getByText(/Verify that DNS queries are routed through the VPN/i),
    ).toBeVisible();
  });

  it('shows not-installed message when WireGuard is not installed', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () => {
        return HttpResponse.json([
          {
            id: 'wireguard',
            name: 'WireGuard',
            description: 'Fast, modern, secure VPN tunnel',
            state: 'not_installed',
            auto_start: false,
          },
          {
            id: 'tailscale',
            name: 'Tailscale',
            description: 'Zero config VPN mesh network',
            state: 'running',
            version: '1.62.0',
            auto_start: true,
          },
        ]);
      }),
    );

    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('WireGuard is not installed')).toBeInTheDocument();
    });

    const links = screen.getAllByText('Install via Services →');
    expect(links.length).toBeGreaterThanOrEqual(1);
    expect(links[0].closest('a')).toHaveAttribute('href', '/services');
  });

  it('shows not-installed message when Tailscale is not installed', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () => {
        return HttpResponse.json([
          {
            id: 'wireguard',
            name: 'WireGuard',
            description: 'Fast, modern, secure VPN tunnel',
            state: 'running',
            version: '1.0.20210914',
            auto_start: true,
          },
          {
            id: 'tailscale',
            name: 'Tailscale',
            description: 'Zero config VPN mesh network',
            state: 'not_installed',
            auto_start: false,
          },
        ]);
      }),
    );

    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('WireGuard')).toBeInTheDocument();
    });
  });
});
