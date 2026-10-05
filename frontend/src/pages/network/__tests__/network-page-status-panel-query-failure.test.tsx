import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { NetworkPageStatusPanel } from '../network-page-status-panel';

// NetworkPage owns the network-status query and hands the panel its outcome.
// A failed GET /network/status used to render "No clients connected": a
// confident statement about the router on a bad link.
describe('NetworkPageStatusPanel when the network status request fails', () => {
  function renderPanel(props: { isError: boolean; error: unknown; onRetry: () => void }) {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return render(
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <NetworkPageStatusPanel
            network={undefined}
            isLoading={false}
            blockedClients={[]}
            {...props}
          />
        </ThemeProvider>
      </QueryClientProvider>,
    );
  }

  it('reports the error instead of an empty client list', async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderPanel({ isError: true, error: new Error('ubus busy'), onRetry });

    expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    expect(screen.queryByText('No clients connected')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('shows the empty state only after a successful query with no clients', () => {
    renderPanel({ isError: false, error: null, onRetry: () => {} });

    expect(screen.getByText('No clients connected')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
