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
import { WifiPage } from '../wifi-page';

function renderWifiPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const wifiRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/wifi',
    component: WifiPage,
  });

  const routeTree = rootRoute.addChildren([wifiRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/wifi'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('WifiPage', () => {
  it('renders current connection section with SSID', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Current Connection')).toBeInTheDocument();
    });

    await waitFor(() => {
      const matches = screen.getAllByText('Hotel_Guest_5G');
      expect(matches.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders connected badge for active connection', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeInTheDocument();
    });
  });

  it('renders saved networks section', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Saved Networks')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Home_WiFi')).toBeInTheDocument();
      expect(screen.getByText('Office_Network')).toBeInTheDocument();
    });
  });

  it('renders scan networks button', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText('Scan Networks')).toBeInTheDocument();
    });
  });

  it('renders signal strength and IP for current connection', async () => {
    renderWifiPage();

    await waitFor(() => {
      expect(screen.getByText(/82%/)).toBeInTheDocument();
      expect(screen.getByText(/192\.168\.1\.105/)).toBeInTheDocument();
    });
  });
});
