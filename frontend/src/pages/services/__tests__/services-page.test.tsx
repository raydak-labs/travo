import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw/http';
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
      const removeButtons = screen.getAllByRole('button', { name: /Remove/ });
      expect(removeButtons.length).toBe(4); // All services
    });
  });

  // The log dialog fires the uninstall stream from its mount effect, so a Remove
  // tap used to start uninstalling a package with no confirm at all.
  it('asks for confirmation before any remove stream request is issued', async () => {
    const user = userEvent.setup();
    const removed: string[] = [];
    server.use(
      http.post(`${API_ROUTES.services.removeStream.replace(':id', ':id')}`, ({ params }) => {
        removed.push(params.id as string);
        return new HttpResponse(JSON.stringify({ type: 'done' }), {
          headers: { 'Content-Type': 'application/x-ndjson' },
        });
      }),
    );

    renderServicesPage();

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /Remove/ }).length).toBe(4);
    });

    await user.click(screen.getByRole('button', { name: 'Remove AdGuard Home' }));

    const dialog = await screen.findByRole('dialog');
    expect(dialog.textContent).toContain('Remove AdGuard Home?');
    expect(removed).toEqual([]);

    await user.click(screen.getByRole('button', { name: 'Remove now' }));

    await waitFor(() => {
      expect(removed).toEqual(['adguardhome']);
    });
  });

  it('cancels the confirmation without starting the uninstall', async () => {
    const user = userEvent.setup();
    const removed: string[] = [];
    server.use(
      http.post(`${API_ROUTES.services.removeStream.replace(':id', ':id')}`, ({ params }) => {
        removed.push(params.id as string);
        return new HttpResponse(JSON.stringify({ type: 'done' }), {
          headers: { 'Content-Type': 'application/x-ndjson' },
        });
      }),
    );

    renderServicesPage();

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /Remove/ }).length).toBe(4);
    });

    await user.click(screen.getByRole('button', { name: 'Remove AdGuard Home' }));
    await user.click(await screen.findByRole('button', { name: 'Cancel' }));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    expect(removed).toEqual([]);
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
