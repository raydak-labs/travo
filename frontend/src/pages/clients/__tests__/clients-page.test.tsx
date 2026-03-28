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
import { ClientsPage } from '../clients-page';

function renderClientsPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });
  const clientsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/clients',
    component: ClientsPage,
  });

  const routeTree = rootRoute.addChildren([clientsRoute]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/clients'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('ClientsPage', () => {
  it('renders heading and client list', async () => {
    renderClientsPage();
    await waitFor(() => {
      expect(screen.getByText('Connected Clients')).toBeInTheDocument();
    });
    // Mock data has "John's Laptop" (alias) and "iPhone-15" (hostname)
    expect(await screen.findByText("John's Laptop")).toBeInTheDocument();
    expect(await screen.findByText('iPhone-15')).toBeInTheDocument();
  });

  it('renders static IP reservations section', async () => {
    renderClientsPage();
    await waitFor(() => {
      expect(screen.getByText('Static IP Reservations')).toBeInTheDocument();
    });
  });

  it('shows search input', async () => {
    renderClientsPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Search by name, IP, or MAC…')).toBeInTheDocument();
    });
  });
});
