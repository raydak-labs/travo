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
import { ServicesPage } from '../services-page';

function renderServicesPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const servicesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services',
    component: ServicesPage,
  });

  const routeTree = rootRoute.addChildren([servicesRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/services'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('ServicesPage', () => {
  it('renders service cards', async () => {
    renderServicesPage();

    await waitFor(() => {
      expect(screen.getByText('Tailscale')).toBeInTheDocument();
      expect(screen.getByText('AdGuard Home')).toBeInTheDocument();
      expect(screen.getByText('WireGuard')).toBeInTheDocument();
      expect(screen.getAllByText('SQM (Traffic Shaping)').length).toBeGreaterThan(0);
    });
  });

  it('shows correct state badges', async () => {
    renderServicesPage();

    await waitFor(() => {
      const runningBadges = screen.getAllByText('Running');
      expect(runningBadges.length).toBe(2); // Tailscale + WireGuard

      const installedBadges = screen.getAllByText('Installed');
      expect(installedBadges.length).toBe(2); // AdGuard Home + SQM
    });
  });

  it('shows Stop button for running services and Start for installed', async () => {
    renderServicesPage();

    await waitFor(() => {
      const stopButtons = screen.getAllByRole('button', { name: 'Stop' });
      expect(stopButtons.length).toBe(2); // Tailscale + WireGuard

      const startButtons = screen.getAllByRole('button', { name: 'Start' });
      expect(startButtons.length).toBe(2); // AdGuard Home + SQM
    });
  });

  it('shows Remove buttons for installed services', async () => {
    renderServicesPage();

    await waitFor(() => {
      const removeButtons = screen.getAllByRole('button', { name: 'Remove' });
      expect(removeButtons.length).toBe(4); // All services
    });
  });

  it('shows service descriptions', async () => {
    renderServicesPage();

    await waitFor(() => {
      expect(screen.getByText('Zero config VPN mesh network')).toBeInTheDocument();
      expect(screen.getByText('Network-wide ad and tracker blocker')).toBeInTheDocument();
      expect(screen.getByText('Fast, modern, secure VPN tunnel')).toBeInTheDocument();
      expect(
        screen.getByText('Smart Queue Management to reduce latency (bufferbloat)'),
      ).toBeInTheDocument();
    });
  });
});
