import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  createRouter,
  createRoute,
  createRootRoute,
  RouterProvider,
  Outlet,
  createMemoryHistory,
} from '@tanstack/react-router';
import { Sidebar } from '../sidebar';
import { ThemeProvider } from '../theme-provider';
import { AppShell } from '../app-shell';
import { useIsMobile } from '@/hooks/use-mobile';
import { useServices } from '@/hooks/use-services';

vi.mock('@/hooks/use-mobile', () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock('@/hooks/use-alerts', () => ({
  useAlerts: vi.fn(() => ({ alerts: [], unreadCount: 0, markAllRead: vi.fn() })),
}));

vi.mock('@/hooks/use-system', () => ({
  useSystemInfo: vi.fn(() => ({ data: null, isError: false })),
  useReboot: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useShutdown: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}));

vi.mock('@/hooks/use-services', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/hooks/use-services')>();
  return {
    ...actual,
    useServices: vi.fn(() => ({ data: undefined, isLoading: false, isError: false })),
  };
});

const mockUseIsMobile = vi.mocked(useIsMobile);
const mockUseServices = vi.mocked(useServices);

const ROUTE_PATHS = [
  '/dashboard-1',
  '/dashboard-2',
  '/wifi',
  '/wifi/advanced',
  '/network',
  '/network/configuration',
  '/network/advanced',
  '/clients',
  '/vpn',
  '/services',
  '/services/tailscale',
  '/services/sqm',
  '/system',
  '/logs',
] as const;

function renderSidebar(currentPath = '/dashboard-2') {
  const rootRoute = createRootRoute({ component: Outlet });

  const routes = ROUTE_PATHS.map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <Sidebar collapsed={false} onToggle={() => {}} />,
    }),
  );

  const routeTree = rootRoute.addChildren(routes);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [currentPath] }),
  });

  return render(<RouterProvider router={router} />);
}

function renderAppShellMobile(currentPath = '/dashboard-2') {
  mockUseIsMobile.mockReturnValue(true);

  const rootRoute = createRootRoute({ component: Outlet });

  const routes = ROUTE_PATHS.map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => (
        <ThemeProvider>
          <AppShell title="Test">
            <div data-testid="page-content">Content</div>
          </AppShell>
        </ThemeProvider>
      ),
    }),
  );

  const routeTree = rootRoute.addChildren(routes);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [currentPath] }),
  });

  return render(<RouterProvider router={router} />);
}

describe('Sidebar', () => {
  beforeEach(() => {
    mockUseIsMobile.mockReturnValue(false);
  });

  it('renders category groups and leaf routes', async () => {
    renderSidebar();
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
      expect(screen.getByText('Dashboard (NEW)')).toBeInTheDocument();
      expect(screen.getByText('WiFi')).toBeInTheDocument();
      expect(screen.getByText('Wireless')).toBeInTheDocument();
      expect(screen.getByText('Network')).toBeInTheDocument();
      expect(screen.getByText('Status')).toBeInTheDocument();
      expect(screen.getByText('Configuration')).toBeInTheDocument();
      expect(screen.getAllByText('Advanced').length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText('Clients')).toBeInTheDocument();
      expect(screen.getByText('VPN')).toBeInTheDocument();
      expect(screen.getByText('Services')).toBeInTheDocument();
      expect(screen.getByText('Installed services')).toBeInTheDocument();
      expect(screen.getByText('Tailscale')).toBeInTheDocument();
      expect(screen.getByText('System')).toBeInTheDocument();
      expect(screen.getByText('Settings')).toBeInTheDocument();
      expect(screen.getByText('Logs')).toBeInTheDocument();
    });
  });

  it('shows SQM under Services when SQM is installed', async () => {
    mockUseServices.mockReturnValueOnce({
      data: [
        {
          id: 'sqm',
          name: 'SQM (Traffic Shaping)',
          description: 'Smart Queue Management',
          state: 'stopped',
          auto_start: false,
        },
      ],
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useServices>);
    renderSidebar();
    await waitFor(() => {
      expect(screen.getByText('SQM')).toBeInTheDocument();
    });
  });

  it('highlights active link', async () => {
    renderSidebar('/dashboard-1');
    await waitFor(() => {
      const dashboardLink = screen.getByRole('link', { name: 'Dashboard' });
      expect(dashboardLink).toHaveClass('bg-blue-50');
    });
  });
});

describe('Mobile Sidebar', () => {
  beforeEach(() => {
    mockUseIsMobile.mockReturnValue(true);
  });

  afterEach(() => {
    mockUseIsMobile.mockReturnValue(false);
  });

  it('shows hamburger menu button on mobile', async () => {
    renderAppShellMobile();
    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });
  });

  it('opens drawer when hamburger is clicked', async () => {
    const user = userEvent.setup();
    renderAppShellMobile();

    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Open menu'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
    });
  });

  it('closes drawer when navigation link is clicked', async () => {
    const user = userEvent.setup();
    renderAppShellMobile('/dashboard-1');

    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Open menu'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'Wireless' }));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('closes drawer when close button is clicked', async () => {
    const user = userEvent.setup();
    renderAppShellMobile();

    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Open menu'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Close menu'));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });
});
