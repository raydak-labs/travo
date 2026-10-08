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
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { TailscaleSection } from '../tailscale-section';

function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  const rootRoute = createRootRoute({ component: Outlet });
  const servicesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services',
    component: TailscaleSection,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([servicesRoute]),
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

// One failed GET used to render "Tailscale is not installed" with an install
// link: a confident claim about the router on a bad link.
describe('TailscaleSection when the install state cannot be determined', () => {
  it('does not claim Tailscale is missing', async () => {
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
    expect(screen.queryByText(/Tailscale is not installed/)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Install via Services/ })).not.toBeInTheDocument();
  });

  it('still shows the install prompt when the service list confirms it is absent', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () =>
        HttpResponse.json([
          {
            id: 'tailscale',
            name: 'Tailscale',
            description: 'Zero config VPN mesh network',
            state: 'not_installed',
            auto_start: false,
          },
        ]),
      ),
      http.get(API_ROUTES.vpn.status, () => HttpResponse.json([])),
    );
    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/Tailscale is not installed/)).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: /Install via Services/ })).toBeInTheDocument();
  });
});
