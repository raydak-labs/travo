import { describe, it, expect, vi } from 'vitest';
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
import { RepeaterWizard } from '@/components/wifi/repeater-wizard';

function renderWizard(open = true, onOpenChange = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute({ component: Outlet });

  const TestComponent = () => (
    <RepeaterWizard open={open} onOpenChange={onOpenChange} />
  );

  const testRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/test',
    component: TestComponent,
  });

  const routeTree = rootRoute.addChildren([testRoute]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/test'] }),
  });

  return render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe('RepeaterWizard', () => {
  it('renders wizard dialog with title and step indicators', async () => {
    renderWizard();

    await waitFor(() => {
      expect(screen.getByText('Repeater Setup Wizard')).toBeInTheDocument();
    });

    expect(screen.getByText('Upstream')).toBeInTheDocument();
    expect(screen.getByText('AP Config')).toBeInTheDocument();
    expect(screen.getByText('Review')).toBeInTheDocument();
  });

  it('shows scan results in step 1', async () => {
    renderWizard();

    await waitFor(() => {
      expect(screen.getByText('Available Networks')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });
  });

  it('selects a network and shows password field', async () => {
    const user = userEvent.setup();
    renderWizard();

    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    // Click connect on the first network
    const connectButtons = screen.getAllByRole('button', { name: 'Connect' });
    await user.click(connectButtons[0]);

    // Should show selected network and password field
    await waitFor(() => {
      expect(screen.getByLabelText('Password')).toBeInTheDocument();
    });

    expect(screen.getByText('Change')).toBeInTheDocument();
  });

  it('proceeds to step 2 after entering password', async () => {
    const user = userEvent.setup();
    renderWizard();

    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    const connectButtons = screen.getAllByRole('button', { name: 'Connect' });
    await user.click(connectButtons[0]);

    await waitFor(() => {
      expect(screen.getByLabelText('Password')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('Password'), 'testpass123');

    const nextButton = screen.getByRole('button', { name: /Next/ });
    await user.click(nextButton);

    // Step 2: AP config
    await waitFor(() => {
      expect(screen.getByLabelText('Same as upstream')).toBeInTheDocument();
    });
  });

  it('shows review step with summary', async () => {
    const user = userEvent.setup();
    renderWizard();

    // Step 1: select network
    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    const connectButtons = screen.getAllByRole('button', { name: 'Connect' });
    await user.click(connectButtons[0]);

    await waitFor(() => {
      expect(screen.getByLabelText('Password')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('Password'), 'testpass123');
    await user.click(screen.getByRole('button', { name: /Next/ }));

    // Step 2: AP config (keep same as upstream)
    await waitFor(() => {
      expect(screen.getByLabelText('Same as upstream')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /Next/ }));

    // Step 3: Review
    await waitFor(() => {
      expect(screen.getByText('Upstream Connection')).toBeInTheDocument();
      expect(screen.getByText('Access Point')).toBeInTheDocument();
      expect(screen.getByText('Apply Configuration')).toBeInTheDocument();
    });
  });

  it('can go back from step 2 to step 1', async () => {
    const user = userEvent.setup();
    renderWizard();

    await waitFor(() => {
      expect(screen.getByText('Hotel_Guest_5G')).toBeInTheDocument();
    });

    // Select an open network (no password needed)
    // Scan list sorted by signal: Hotel_5G(82), Hotel_2G(65), CafeNet(55), Airport(45), Starbucks(30), Neighbor(18)
    // Airport_Free_WiFi (open) is at index 3
    const connectButtons = screen.getAllByRole('button', { name: 'Connect' });
    await user.click(connectButtons[3]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Next/ })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /Next/ }));

    // Step 2
    await waitFor(() => {
      expect(screen.getByLabelText('Same as upstream')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /Back/ }));

    // Back to step 1
    await waitFor(() => {
      expect(screen.getByText('Change')).toBeInTheDocument();
    });
  });

  it('does not render when closed', () => {
    renderWizard(false);
    expect(screen.queryByText('Repeater Setup Wizard')).not.toBeInTheDocument();
  });
});
