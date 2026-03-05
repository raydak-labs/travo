import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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

  const routeTree = rootRoute.addChildren([vpnRoute]);

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
  it('renders VPN overview section', async () => {
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('VPN Overview')).toBeInTheDocument();
    });
  });

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

  it('shows Tailscale section', async () => {
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('Tailscale')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Logged In')).toBeInTheDocument();
    });
  });

  it('has toggle switches for WireGuard and Tailscale', async () => {
    renderVpnPage();

    await waitFor(() => {
      const toggles = screen.getAllByRole('checkbox');
      expect(toggles.length).toBeGreaterThanOrEqual(2);
    });
  });

  it('shows WireGuard peers', async () => {
    renderVpnPage();

    await waitFor(() => {
      const matches = screen.getAllByText(/vpn\.example\.com:51820/);
      expect(matches.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows Tailscale IP and hostname', async () => {
    renderVpnPage();

    await waitFor(() => {
      expect(screen.getByText('100.100.1.42')).toBeInTheDocument();
      expect(screen.getByText('gl-mt3000')).toBeInTheDocument();
    });
  });
});
