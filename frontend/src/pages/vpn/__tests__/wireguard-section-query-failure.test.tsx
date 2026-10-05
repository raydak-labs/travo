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
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { WireguardSection } from '../wireguard-section';

function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  const rootRoute = createRootRoute({ component: Outlet });
  const vpnRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/vpn',
    component: WireguardSection,
  });
  const servicesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services',
    component: () => <div>Services page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([vpnRoute, servicesRoute]),
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

// One failed GET used to render "WireGuard is not installed" plus an install
// link — a confident claim about the router plus an action that cannot work.
describe('WireguardSection when the install state cannot be determined', () => {
  it('does not claim WireGuard is missing', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
      http.get(API_ROUTES.vpn.status, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderSection();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.queryByText(/WireGuard is not installed/)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Install via Services/ })).not.toBeInTheDocument();
  });

  it('still shows the install prompt when the service list confirms it is absent', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () =>
        HttpResponse.json([
          {
            id: 'wireguard',
            name: 'WireGuard',
            description: 'Fast, modern, secure VPN tunnel',
            state: 'not_installed',
            auto_start: false,
          },
        ]),
      ),
      http.get(API_ROUTES.vpn.status, () => HttpResponse.json([])),
    );
    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/WireGuard is not installed/)).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: /Install via Services/ })).toBeInTheDocument();
  });
});
