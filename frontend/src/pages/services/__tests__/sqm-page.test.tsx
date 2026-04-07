import { describe, it, expect, vi, beforeEach } from 'vitest';
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
import { useServices } from '@/hooks/use-services';
import { mockServices } from '@/mocks/data';
import { SQMPage } from '../sqm-page';

vi.mock('@/hooks/use-services', () => ({
  useServices: vi.fn(),
}));

const mockUseServices = vi.mocked(useServices);

function renderSQMPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: '/services/sqm',
    component: SQMPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ['/services/sqm'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('SQMPage', () => {
  beforeEach(() => {
    mockUseServices.mockReturnValue({
      data: mockServices.map((s) => (s.id === 'sqm' ? { ...s, state: 'not_installed' } : s)),
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useServices>);
  });

  it('prompts to install when SQM is not installed', async () => {
    renderSQMPage();

    await waitFor(() => {
      expect(screen.getByText(/Install SQM from Installed services/i)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /Go to Installed services/i })).toBeInTheDocument();
    });
  });
});
