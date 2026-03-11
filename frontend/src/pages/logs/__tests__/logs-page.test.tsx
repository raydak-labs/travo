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
import { LogsPage } from '../logs-page';

function renderLogsPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const logsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/logs',
    component: LogsPage,
  });

  const routeTree = rootRoute.addChildren([logsRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/logs'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('LogsPage', () => {
  it('renders tab buttons for System Log and Kernel Log', async () => {
    renderLogsPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'System Log' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Kernel Log' })).toBeInTheDocument();
    });
  });

  it('renders log content area with data after loading', async () => {
    renderLogsPage();

    await waitFor(() => {
      expect(screen.getByTestId('log-content')).toBeInTheDocument();
    });

    // Verify the log area rendered (either with content or empty-state)
    const logContent = screen.getByTestId('log-content');
    expect(logContent).toBeInTheDocument();
  });

  it('renders filter input and refresh button', async () => {
    renderLogsPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Filter logs…')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Refresh logs' })).toBeInTheDocument();
    });
  });
});
