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

// A failed service list used to fall back to `services = []`, which rendered
// "No services available" — a confident claim about the router's packages.
describe('ServicesPage when the service list cannot be loaded', () => {
  it('states the failure instead of claiming no services are available', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );

    renderServicesPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.queryByText('No services available')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Retry/i })).toBeInTheDocument();
  });
});
