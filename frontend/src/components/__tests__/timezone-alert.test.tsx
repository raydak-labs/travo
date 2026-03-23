import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';
import { TimezoneAlert } from '../timezone-alert';

function renderWithProviders() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: TimezoneAlert,
  });
  const systemRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/system',
    component: () => <div>System Page</div>,
  });

  const routeTree = rootRoute.addChildren([indexRoute, systemRoute]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
});

describe('TimezoneAlert', () => {
  it('shows alert when device timezone differs from browser', async () => {
    // Mock device timezone as UTC — browser will be something else in most CI/test environments
    // We override Intl to ensure predictable behavior
    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Europe/Berlin' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
    );

    renderWithProviders();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText('UTC', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Europe/Berlin', { exact: false })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();

    vi.unstubAllGlobals();
  });

  it('does not show alert when timezones match', async () => {
    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'UTC' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
    );

    renderWithProviders();

    // Wait for the query to settle, then confirm no alert
    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    vi.unstubAllGlobals();
  });

  it('dismisses alert and stores in sessionStorage', async () => {
    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'America/New_York' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
    );

    renderWithProviders();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByLabelText('Dismiss timezone alert'));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(sessionStorage.getItem('timezone-alert-dismissed')).toBe('true');

    vi.unstubAllGlobals();
  });

  it('does not show alert if previously dismissed', async () => {
    sessionStorage.setItem('timezone-alert-dismissed', 'true');

    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Europe/Berlin' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
    );

    renderWithProviders();

    // Give time for query to resolve
    await new Promise((r) => setTimeout(r, 100));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    vi.unstubAllGlobals();
  });

  it('auto-sets timezone and dismisses alert when browser timezone is known', async () => {
    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Asia/Tokyo' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
      http.put(API_ROUTES.system.timezone, () => HttpResponse.json({ status: 'ok' })),
    );

    renderWithProviders();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Update' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    vi.unstubAllGlobals();
  });

  it('navigates to system page when Update is clicked for unknown timezone', async () => {
    const originalIntl = globalThis.Intl;
    const mockDateTimeFormat = vi.fn().mockImplementation(() => ({
      resolvedOptions: () => ({ timeZone: 'Pacific/Tahiti' }),
    }));
    Object.assign(mockDateTimeFormat, originalIntl.DateTimeFormat);
    vi.stubGlobal('Intl', { ...originalIntl, DateTimeFormat: mockDateTimeFormat });

    server.use(
      http.get(API_ROUTES.system.timezone, () =>
        HttpResponse.json({ zonename: 'UTC', timezone: 'UTC0' }),
      ),
    );

    renderWithProviders();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Update' }));

    await waitFor(() => {
      expect(screen.getByText('System Page')).toBeInTheDocument();
    });

    vi.unstubAllGlobals();
  });
});
