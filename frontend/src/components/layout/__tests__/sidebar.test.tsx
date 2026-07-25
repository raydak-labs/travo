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
  '/dashboard',
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

function renderSidebar(currentPath = '/dashboard') {
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

  const view = render(<RouterProvider router={router} />);
  return { ...view, router };
}

function renderAppShellMobile(currentPath = '/dashboard') {
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
    mockUseServices.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useServices>);
    localStorage.clear();
  });

  it('renders category groups and leaf routes', async () => {
    renderSidebar();
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
      expect(screen.getByText('WiFi')).toBeInTheDocument();
      expect(screen.getByText('Network')).toBeInTheDocument();
      expect(screen.getByText('Clients')).toBeInTheDocument();
      expect(screen.getByText('VPN')).toBeInTheDocument();
      expect(screen.getByText('Services')).toBeInTheDocument();
      expect(screen.getByText('System')).toBeInTheDocument();
    });
  });

  it('keeps WiFi children collapsed on dashboard with empty storage', async () => {
    renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'WiFi' })).toBeInTheDocument();
    });
    const wifiToggle = screen.getByRole('button', { name: 'Toggle WiFi menu' });
    expect(wifiToggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: 'Connect' })).not.toBeInTheDocument();
  });

  it('keeps all groups collapsed on dashboard with empty storage', async () => {
    renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'WiFi' })).toBeInTheDocument();
    });
    for (const name of ['WiFi', 'Network', 'Services', 'System']) {
      expect(screen.getByRole('button', { name: `Toggle ${name} menu` })).toHaveAttribute(
        'aria-expanded',
        'false',
      );
    }
  });

  it('auto-expands WiFi group on /wifi/advanced with empty storage', async () => {
    renderSidebar('/wifi/advanced');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Toggle WiFi menu' })).toHaveAttribute(
        'aria-expanded',
        'true',
      );
    });
    const advanced = screen.getByRole('link', { name: 'Advanced' });
    expect(advanced).toHaveClass('bg-blue-50');
    expect(advanced).toHaveAttribute('href', '/wifi/advanced');
  });

  it('marks only the Advanced leaf as current on /wifi/advanced', async () => {
    renderSidebar('/wifi/advanced');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Advanced' })).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Advanced' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(screen.queryByRole('link', { name: 'Connect' })).not.toBeInTheDocument();
  });

  it('highlights Network parent on Status default without submenu duplicate', async () => {
    renderSidebar('/network');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Network' })).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Network' })).toHaveClass('bg-blue-50');
    expect(screen.queryByRole('link', { name: 'Status' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Internet & LAN' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Advanced' })).toBeInTheDocument();
  });

  it('navigates to Connect when WiFi label is clicked without opening via arrow', async () => {
    const user = userEvent.setup();
    const { router } = renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'WiFi' })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'WiFi' }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/wifi');
      expect(screen.getByRole('button', { name: 'Toggle WiFi menu' })).toHaveAttribute(
        'aria-expanded',
        'true',
      );
    });
  });

  it('navigates to Status when Network label is clicked', async () => {
    const user = userEvent.setup();
    const { router } = renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Network' })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'Network' }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/network');
    });
  });

  it('expands WiFi group when navigating from dashboard to /wifi', async () => {
    const { router } = renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Toggle WiFi menu' })).toHaveAttribute(
        'aria-expanded',
        'false',
      );
    });

    await router.navigate({ to: '/wifi' });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Toggle WiFi menu' })).toHaveAttribute(
        'aria-expanded',
        'true',
      );
      expect(screen.getByRole('link', { name: 'WiFi' })).toHaveClass('bg-blue-50');
      expect(screen.queryByRole('link', { name: 'Connect' })).not.toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Advanced' })).toBeInTheDocument();
    });
  });

  it('shows Advanced (not Connect) after opening WiFi group via arrow', async () => {
    const user = userEvent.setup();
    renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'WiFi' })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: 'Toggle WiFi menu' }));
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Advanced' })).toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'Connect' })).not.toBeInTheDocument();
    });
  });

  it('shows SQM under Services when SQM is installed', async () => {
    const user = userEvent.setup();
    mockUseServices.mockReturnValue({
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
      expect(screen.getByRole('link', { name: 'Services' })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: 'Toggle Services menu' }));
    await waitFor(() => {
      expect(screen.getByText('SQM')).toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'Apps' })).not.toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Tailscale' })).toBeInTheDocument();
    });
  });

  it('highlights active link', async () => {
    renderSidebar('/dashboard');
    await waitFor(() => {
      const dashboardLink = screen.getByRole('link', { name: 'Dashboard' });
      expect(dashboardLink).toHaveClass('bg-blue-50');
    });
  });
});

describe('Mobile Sidebar', () => {
  beforeEach(() => {
    mockUseIsMobile.mockReturnValue(true);
    localStorage.clear();
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
    renderAppShellMobile('/dashboard');

    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Open menu'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'WiFi' }));

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
