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
import { NetworkPage } from '../network-page';

function renderNetworkPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const networkRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/network',
    component: NetworkPage,
  });

  const routeTree = rootRoute.addChildren([networkRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/network'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('NetworkPage', () => {
  it('renders WAN information', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('WAN Configuration')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getAllByText('192.168.1.105').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('192.168.1.1').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders LAN configuration', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('LAN Configuration')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getAllByText('192.168.8.1').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders clients table', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
    });

    await waitFor(() => {
      // Clients with aliases show the alias as primary name
      expect(screen.getByText("John's Laptop")).toBeInTheDocument();
      expect(screen.getByText('iPhone-15')).toBeInTheDocument();
      expect(screen.getByText('Living Room iPad')).toBeInTheDocument();
    });
  });

  it('shows internet connectivity status', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('WAN Status')).toBeInTheDocument();
      expect(screen.getByText('Internet Connected')).toBeInTheDocument();
    });
  });

  it('shows DNS servers', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('8.8.8.8, 8.8.4.4')).toBeInTheDocument();
    });
  });

  it('renders auto-detect WAN type button', async () => {
    renderNetworkPage();

    await waitFor(() => {
      expect(screen.getByText('Auto-detect WAN Type')).toBeInTheDocument();
    });
  });
});
