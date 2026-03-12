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

vi.mock('@/hooks/use-mobile', () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock('@/hooks/use-alerts', () => ({
  useAlerts: vi.fn(() => ({ alerts: [], unreadCount: 0, markAllRead: vi.fn() })),
}));

vi.mock('@/hooks/use-system', () => ({
  useSystemInfo: vi.fn(() => ({ data: null, isError: false })),
  useReboot: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}));

const mockUseIsMobile = vi.mocked(useIsMobile);

function renderSidebar(currentPath = '/dashboard') {
  const rootRoute = createRootRoute({ component: Outlet });

  const routes = ['/dashboard', '/wifi', '/network', '/vpn', '/services', '/system'].map((path) =>
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

function renderAppShellMobile(currentPath = '/dashboard') {
  mockUseIsMobile.mockReturnValue(true);

  const rootRoute = createRootRoute({ component: Outlet });

  const routes = ['/dashboard', '/wifi', '/network', '/vpn', '/services', '/system'].map((path) =>
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

  it('renders all navigation links', async () => {
    renderSidebar();
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
      expect(screen.getByText('WiFi')).toBeInTheDocument();
      expect(screen.getByText('Network')).toBeInTheDocument();
      expect(screen.getByText('VPN')).toBeInTheDocument();
      expect(screen.getByText('Services')).toBeInTheDocument();
      expect(screen.getByText('System')).toBeInTheDocument();
    });
  });

  it('highlights active link', async () => {
    renderSidebar('/dashboard');
    await waitFor(() => {
      const dashboardLink = screen.getByText('Dashboard').closest('a');
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
    renderAppShellMobile('/dashboard');

    await waitFor(() => {
      expect(screen.getByLabelText('Open menu')).toBeInTheDocument();
    });

    // Open drawer
    await user.click(screen.getByLabelText('Open menu'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Click a nav link
    await user.click(screen.getByText('WiFi'));

    // Drawer should close (route changes, new AppShell mounts with mobileOpen=false)
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
