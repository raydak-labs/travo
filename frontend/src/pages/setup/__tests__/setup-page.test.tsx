import { describe, it, expect, beforeEach } from 'vitest';
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
import { ThemeProvider } from '@/components/layout/theme-provider';
import { SetupPage } from '../setup-page';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';

function renderSetup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const setupRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/setup',
    component: SetupPage,
  });

  const dashboardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/dashboard',
    component: () => <div>Dashboard</div>,
  });

  const routeTree = rootRoute.addChildren([setupRoute, dashboardRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/setup'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('SetupPage', () => {
  beforeEach(() => {
    localStorage.setItem('openwrt-auth-token', 'test-token');
  });

  it('renders the welcome step', async () => {
    renderSetup();
    await waitFor(() => {
      expect(screen.getByText('Welcome to your Travel Router')).toBeInTheDocument();
    });
    expect(screen.getByText('Get Started')).toBeInTheDocument();
  });

  it('navigates to password step when clicking Get Started', async () => {
    const user = userEvent.setup();
    renderSetup();

    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });
  });

  it('shows password validation errors', async () => {
    const user = userEvent.setup();
    renderSetup();

    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByPlaceholderText('Enter new password (min 8 chars)');
    await user.type(newPasswordInput, 'short');

    expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
  });

  it('can skip password step', async () => {
    const user = userEvent.setup();
    renderSetup();

    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Connect to WiFi')).toBeInTheDocument();
    });
  });

  it('shows wifi scan results', async () => {
    const user = userEvent.setup();
    renderSetup();

    // Navigate to WiFi step
    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Connect to WiFi')).toBeInTheDocument();
    });

    // Should show scanned networks
    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });
  });

  it('can navigate back from wifi step', async () => {
    const user = userEvent.setup();
    renderSetup();

    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Connect to WiFi')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });
  });

  it('shows the complete step and can finish setup', async () => {
    const user = userEvent.setup();

    server.use(
      http.post(API_ROUTES.system.setupComplete, () => {
        return HttpResponse.json({ status: 'ok' });
      }),
    );

    renderSetup();

    // Navigate through all steps
    await waitFor(() => {
      expect(screen.getByText('Get Started')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Get Started'));

    await waitFor(() => {
      expect(screen.getByText('Change Default Password')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Connect to WiFi')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Configure Access Point')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Skip for now'));

    await waitFor(() => {
      expect(screen.getByText('Setup Complete!')).toBeInTheDocument();
    });

    expect(screen.getByText('Go to Dashboard')).toBeInTheDocument();
  });

  it('displays step indicators', async () => {
    renderSetup();
    await waitFor(() => {
      expect(screen.getByText('Welcome')).toBeInTheDocument();
      expect(screen.getByText('Password')).toBeInTheDocument();
      expect(screen.getByText('WiFi')).toBeInTheDocument();
      expect(screen.getByText('Access Point')).toBeInTheDocument();
      expect(screen.getByText('Complete')).toBeInTheDocument();
    });
  });
});
