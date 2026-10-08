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
import { SystemPage } from '../system-page';

function renderSystemPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });
  const systemRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/system',
    component: SystemPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([systemRoute]),
    history: createMemoryHistory({ initialEntries: ['/system'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

// A failed NTP or alert-threshold read used to fall through to the form
// defaults, so the page stated the router's configuration without ever having
// received it.
describe('SystemPage when a configuration read fails', () => {
  it('states the NTP failure instead of showing the form defaults', async () => {
    server.use(
      http.get(API_ROUTES.system.ntp, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );

    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('NTP Configuration')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getAllByRole('alert').length).toBeGreaterThan(0);
    });
    expect(screen.queryByRole('button', { name: 'Edit NTP Settings' })).not.toBeInTheDocument();
  });

  it('states the alert threshold failure instead of showing 90/90/90 defaults', async () => {
    server.use(
      http.get(API_ROUTES.system.alertThresholds, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );

    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('Alert Thresholds')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getAllByRole('alert').length).toBeGreaterThan(0);
    });
    expect(screen.queryByRole('button', { name: /Save Thresholds/ })).not.toBeInTheDocument();
  });

  it('keeps the AdGuard password card visible when the service list fails', async () => {
    server.use(
      http.get(API_ROUTES.services.list, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );

    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('AdGuard Password')).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: 'Change' })).not.toBeInTheDocument();
  });
});
