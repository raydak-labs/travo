import { describe, it, expect } from 'vitest';
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
import { ThemeProvider } from '@/components/layout/theme-provider';
import { SystemPage } from '../system-page';

function renderSystemPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const systemRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/system',
    component: SystemPage,
  });

  const routeTree = rootRoute.addChildren([systemRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/system'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('SystemPage', () => {
  it('renders system info', async () => {
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('System Information')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('GL-MT3000')).toBeInTheDocument();
      expect(screen.getByText('GL.iNet GL-MT3000 (Beryl AX)')).toBeInTheDocument();
      expect(screen.getByText('OpenWrt 23.05.3')).toBeInTheDocument();
      expect(screen.getByText('5.15.150')).toBeInTheDocument();
    });
  });

  it('renders uptime', async () => {
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('Uptime')).toBeInTheDocument();
    });

    await waitFor(() => {
      // 86432 seconds = 1 day, 0 hours, 0 minutes (roughly)
      expect(screen.getByText(/1 day/)).toBeInTheDocument();
    });
  });

  it('renders system stats bars', async () => {
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('System Stats')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('CPU')).toBeInTheDocument();
      expect(screen.getByText('Memory')).toBeInTheDocument();
      expect(screen.getAllByText(/Storage/).length).toBeGreaterThan(0);
    });

    await waitFor(() => {
      const progressBars = screen.getAllByRole('progressbar');
      expect(progressBars.length).toBe(3);
    });
  });

  it('shows CPU stats with percentage', async () => {
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText(/12\.5%/)).toBeInTheDocument();
    });
  });

  it('shows reboot button after expanding Danger Zone', async () => {
    const user = userEvent.setup();
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Danger Zone/i })).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: 'Reboot' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Danger Zone/i }));
    expect(screen.getByRole('button', { name: 'Reboot' })).toBeVisible();
  });

  it('collapses maintenance, firmware, and SSH by default', async () => {
    const user = userEvent.setup();
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('System Information')).toBeInTheDocument();
      expect(screen.getByText('Time & Timezone')).toBeInTheDocument();
      expect(screen.getByText('Change Password')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /Backup & Restore/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Firmware Upgrade/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /SSH Public Keys/i })).toBeInTheDocument();
    expect(screen.queryByText('Download Backup')).not.toBeInTheDocument();
    expect(screen.queryByText('Select Firmware Image')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Backup & Restore/i }));
    expect(screen.getByText('Download Backup')).toBeVisible();
  });

  it('renders Quick Links section', async () => {
    renderSystemPage();

    await waitFor(() => {
      expect(screen.getByText('Quick Links')).toBeInTheDocument();
    });
  });

  it('shows LuCI link with port 8080', async () => {
    renderSystemPage();

    await waitFor(() => {
      const luciLink = screen.getByText('LuCI Web Interface').closest('a');
      expect(luciLink).toHaveAttribute('href', expect.stringContaining(':8080'));
    });
  });
});
