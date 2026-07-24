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
      expect(screen.getAllByText('GL-MT3000').length).toBeGreaterThan(0);
    });
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Ethernet (WAN)' })).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'Repeater (WiFi)' })).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'USB Tethering' })).toBeInTheDocument();
    });
  });

  it('forces dark SourceCard chrome for light-mode contrast', async () => {
    renderDashboard();
    const ethernet = await screen.findByRole('heading', { name: 'Ethernet (WAN)' });
    const card = ethernet.closest('[class*="bg-slate-900"]');
    expect(card).toBeTruthy();
    expect(card?.className).toMatch(/border-slate-700/);
    expect(card?.className).toMatch(/bg-slate-900/);
    expect(ethernet.className).toMatch(/text-slate-100/);
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
    const reachable = screen.getByText('Reachable');
    expect(reachable.className).toMatch(/text-emerald-600/);
    expect(reachable.className).toMatch(/dark:text-emerald-400/);
    const vpnState = screen.getByText(/^(On|Off)$/);
    if (vpnState.textContent === 'Off') {
      expect(vpnState.className).toMatch(/text-gray-500 dark:text-gray-400/);
      expect(vpnState.className).toMatch(/dark:text-gray-400/);
    } else {
      expect(vpnState.className).toMatch(/text-emerald-600/);
      expect(vpnState.className).toMatch(/dark:text-emerald-400/);
    }
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

  it('uses stacked topology on small screens and horizontal on md+', async () => {
    renderDashboard();
    const mobile = await screen.findByTestId('topology-mobile');
    const desktop = screen.getByTestId('topology-desktop');
    expect(mobile.className).toMatch(/md:hidden/);
    expect(desktop.className).toMatch(/hidden/);
    expect(desktop.className).toMatch(/md:flex/);
    await waitFor(() => {
      expect(screen.getAllByText('Internet').length).toBeGreaterThan(0);
    });
  });

  it('lets connection detail values wrap instead of hard-truncating SSIDs', async () => {
    renderDashboard();
    const ssid = await screen.findAllByText('Hotel_Guest_5G');
    const value = ssid.find((el) => el.className.includes('font-mono'));
    expect(value).toBeTruthy();
    expect(value!.className).not.toMatch(/max-w-\[80px\]/);
    expect(value!.className).toMatch(/break-words|break-all|min-w-0/);
    const row = value!.parentElement;
    expect(row?.className).toMatch(/flex-col/);
  });
});
