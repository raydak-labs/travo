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

  const systemRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/system',
    component: () => <div>System</div>,
  });

  const routeTree = rootRoute.addChildren([dashboardRoute, systemRoute]);

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
  it('renders topology and connection cards', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('GL-MT3000')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Ethernet (WAN)' })).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'Repeater (WiFi)' })).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'USB Tethering' })).toBeInTheDocument();
    });
  });

  it('shows repeater details when WiFi is connected', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getAllByText('Hotel_Guest_5G').length).toBeGreaterThan(0);
    });
  });

  it('shows quick status and link to system settings', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Quick status')).toBeInTheDocument();
      expect(screen.getByText('Reachable')).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: /Device details and settings/i })).toHaveAttribute(
      'href',
      '/system',
    );
  });

  it('renders quick actions', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Quick Actions')).toBeInTheDocument();
      expect(screen.getAllByRole('button', { name: /WiFi/i }).length).toBeGreaterThan(0);
      expect(screen.getAllByRole('button', { name: /VPN/i }).length).toBeGreaterThan(0);
      expect(screen.getAllByRole('button', { name: /Reboot/i }).length).toBeGreaterThan(0);
    });
  });

  it('renders network throughput chart section', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Network Throughput')).toBeInTheDocument();
    });
  });
});
