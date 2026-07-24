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
      expect(screen.getByText('WiFi')).toBeInTheDocument();
    });
    const wifiTrigger = screen.getByRole('button', { name: /WiFi/i });
    expect(wifiTrigger).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: 'Connect' })).not.toBeInTheDocument();
  });

  it('keeps all groups collapsed on dashboard with empty storage', async () => {
    renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByText('WiFi')).toBeInTheDocument();
    });
    for (const name of [/WiFi/i, /Network/i, /Services/i, /System/i]) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('aria-expanded', 'false');
    }
  });

  it('auto-expands WiFi group on /wifi/advanced with empty storage', async () => {
    renderSidebar('/wifi/advanced');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /WiFi/i })).toHaveAttribute(
        'aria-expanded',
        'true',
      );
    });
    const advanced = screen.getByRole('link', { name: 'Extras' });
    expect(advanced).toHaveClass('bg-blue-50');
    expect(advanced).toHaveAttribute('href', '/wifi/advanced');
  });

  it('marks only the exact leaf as current on /wifi/advanced', async () => {
    renderSidebar('/wifi/advanced');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Extras' })).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Extras' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(screen.getByRole('link', { name: 'Connect' })).not.toHaveAttribute(
      'aria-current',
    );
  });

  it('marks only Internet & LAN as current on /network/configuration', async () => {
    renderSidebar('/network/configuration');
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Internet & LAN' })).toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Internet & LAN' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(screen.getByRole('link', { name: 'Status' })).not.toHaveAttribute(
      'aria-current',
    );
  });

  it('expands WiFi group when navigating from dashboard to /wifi', async () => {
    const { router } = renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /WiFi/i })).toHaveAttribute(
        'aria-expanded',
        'false',
      );
    });

    await router.navigate({ to: '/wifi' });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /WiFi/i })).toHaveAttribute(
        'aria-expanded',
        'true',
      );
      expect(screen.getByRole('link', { name: 'Connect' })).toHaveClass('bg-blue-50');
    });
  });

  it('shows Connect label after opening WiFi group', async () => {
    const user = userEvent.setup();
    renderSidebar('/dashboard');
    await waitFor(() => {
      expect(screen.getByText('WiFi')).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /WiFi/i }));
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Connect' })).toBeInTheDocument();
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
      expect(screen.getByText('Services')).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /Services/i }));
    await waitFor(() => {
      expect(screen.getByText('SQM')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Apps' })).toBeInTheDocument();
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

    await user.click(screen.getByRole('button', { name: /WiFi/i }));
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Connect' })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('link', { name: 'Connect' }));

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
