import { describe, it, expect, vi, beforeEach } from 'vitest';
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
import { DashboardPage } from '../dashboard-page';

// Mock WebSocket for BandwidthChart
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = MockWebSocket.OPEN;
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  close = vi.fn();
  send = vi.fn();
}

beforeEach(() => {
  vi.stubGlobal('WebSocket', MockWebSocket);
});

function renderDashboard() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const dashboardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/dashboard',
    component: DashboardPage,
  });

  const networkRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/network',
    component: () => <div>Network</div>,
  });

  const routeTree = rootRoute.addChildren([dashboardRoute, networkRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/dashboard'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('DashboardPage', () => {
  it('renders all dashboard cards', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Connection Status')).toBeInTheDocument();
      expect(screen.getByText('WAN Source')).toBeInTheDocument();
      expect(screen.getByText('VPN Status')).toBeInTheDocument();
      expect(screen.getByText('System Stats')).toBeInTheDocument();
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
      expect(screen.getByText('Data Usage')).toBeInTheDocument();
    });
  });

  it('shows connection status', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });
  });

  it('shows system stats', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('CPU')).toBeInTheDocument();
      expect(screen.getByText('Memory')).toBeInTheDocument();
    });
  });

  it('shows VPN status', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('wireguard')).toBeInTheDocument();
      expect(screen.getByText('vpn.example.com:51820')).toBeInTheDocument();
    });
  });

  it('renders quick actions', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Quick Actions')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Restart WiFi/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Toggle VPN/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Reboot System/i })).toBeInTheDocument();
    });
  });

  it('renders bandwidth chart section', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('System Usage')).toBeInTheDocument();
    });
  });
});
