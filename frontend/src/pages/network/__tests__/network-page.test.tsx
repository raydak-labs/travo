import { describe, it, expect } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createRouter,
  createRoute,
  createRootRoute,
  RouterProvider,
  Outlet,
  createMemoryHistory,
} from '@tanstack/react-router';
import { http, HttpResponse } from 'msw';
import { API_ROUTES, type Client } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { mockNetworkStatus } from '@/mocks/data';
import { server } from '@/mocks/server';
import { NetworkPage } from '../network-page';

function renderNetworkPage(initialPath = '/network') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const networkConfigurationRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/network/configuration',
    component: NetworkPage,
  });
  const networkAdvancedRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/network/advanced',
    component: NetworkPage,
  });
  const networkRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/network',
    component: NetworkPage,
  });
  const clientsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/clients',
    component: () => <div>Clients page</div>,
  });

  const routeTree = rootRoute.addChildren([
    networkConfigurationRoute,
    networkAdvancedRoute,
    networkRoute,
    clientsRoute,
  ]);

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

function makeClient(index: number): Client {
  const n = String(index).padStart(2, '0');
  return {
    ip_address: `192.168.8.${100 + index}`,
    mac_address: `AA:BB:CC:DD:EE:${n}`,
    hostname: `Client-${n}`,
    interface_name: 'br-lan',
    rx_bytes: 1000,
    tx_bytes: 500,
    connected_since: '2026-03-04T08:00:00Z',
  };
}

describe('NetworkPage', () => {
  it('has no in-page tablist mirroring sidebar', async () => {
    renderNetworkPage('/network');

    await waitFor(() => {
      expect(screen.getByText('WAN Status')).toBeInTheDocument();
    });
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('tab', { name: /Status|Configuration|Advanced/i }),
    ).not.toBeInTheDocument();
  });

  it('shows Status panel content on /network', async () => {
    renderNetworkPage('/network');

    await waitFor(() => {
      expect(screen.getByText('WAN Status')).toBeInTheDocument();
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
    });
    expect(screen.queryByText('LAN Configuration')).not.toBeInTheDocument();
    expect(screen.queryByText(/Dynamic DNS/)).not.toBeInTheDocument();
  });

  it('shows Setup panel content on /network/configuration', async () => {
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('LAN Configuration')).toBeInTheDocument();
    });
    expect(screen.queryByText('WAN Status')).not.toBeInTheDocument();
    expect(screen.queryByText(/Dynamic DNS/)).not.toBeInTheDocument();
  });

  it('shows Advanced panel content on /network/advanced', async () => {
    renderNetworkPage('/network/advanced');

    await waitFor(() => {
      expect(screen.getByText('Connection Failover')).toBeInTheDocument();
    });
    expect(screen.queryByText('WAN Status')).not.toBeInTheDocument();
    expect(screen.queryByText('LAN Configuration')).not.toBeInTheDocument();
  });

  it('collapses secondary Setup sections by default', async () => {
    const user = userEvent.setup();
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('WAN Configuration')).toBeInTheDocument();
      expect(screen.getByText('LAN Configuration')).toBeInTheDocument();
      expect(screen.getByText('Network Interfaces')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /DHCP & DNS/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /DHCP Leases/i })).toBeInTheDocument();
    expect(screen.queryByText('DHCP Configuration')).not.toBeInTheDocument();
    expect(screen.queryByText(/No active leases|Expires/i)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /DHCP & DNS/i }));
    expect(screen.getByText('DHCP Configuration')).toBeVisible();
  });

  it('keeps Failover open and collapses other Advanced sections', async () => {
    const user = userEvent.setup();
    renderNetworkPage('/network/advanced');

    await waitFor(() => {
      expect(screen.getByText('Connection Failover')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /Firewall/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Speed Test/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Run Speed Test/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Speed Test/i }));
    expect(screen.getByRole('button', { name: /Run Speed Test/i })).toBeVisible();
  });

  it('renders WAN information', async () => {
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('WAN Configuration')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getAllByText('192.168.1.105').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('192.168.1.1').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders LAN configuration', async () => {
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('LAN Configuration')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getAllByText('192.168.8.1').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders clients table', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText("John's Laptop")).toBeInTheDocument();
      expect(screen.getByText('iPhone-15')).toBeInTheDocument();
      expect(screen.getByText('Living Room iPad')).toBeInTheDocument();
    });
  });

  it('shows internet connectivity status', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('WAN Status')).toBeInTheDocument();
      expect(screen.getByText('Internet Connected')).toBeInTheDocument();
    });
  });

  it('shows DNS servers', async () => {
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('8.8.8.8, 8.8.4.4')).toBeInTheDocument();
    });
  });

  it('renders auto-detect WAN type button', async () => {
    renderNetworkPage('/network/configuration');

    await waitFor(() => {
      expect(screen.getByText('Auto-detect WAN Type')).toBeInTheDocument();
    });
  });

  it('previews at most 5 clients and links to /clients', async () => {
    const manyClients = Array.from({ length: 7 }, (_, i) => makeClient(i + 1));
    server.use(
      http.get(API_ROUTES.network.status, () =>
        HttpResponse.json({ ...mockNetworkStatus, clients: manyClients }),
      ),
    );

    renderNetworkPage('/network');

    await waitFor(() => {
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
    });

    await waitFor(() => {
      const table = screen.getByRole('table');
      const bodyRows = within(table).getAllByRole('row').slice(1);
      expect(bodyRows).toHaveLength(5);
    });

    const viewAll = screen.getByRole('link', { name: /view all/i });
    expect(viewAll).toHaveAttribute('href', '/clients');
  });
});
