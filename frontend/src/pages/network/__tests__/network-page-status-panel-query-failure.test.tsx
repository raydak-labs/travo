import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { mockNetworkStatus } from '@/mocks/data';
import { server } from '@/mocks/server';
import { NetworkPageStatusPanel } from '../network-page-status-panel';

function renderPanel() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <NetworkPageStatusPanel network={undefined} isLoading={false} blockedClients={[]} />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

// A failed GET /network/status used to render "No clients connected": a
// confident statement about the router on a bad link.
describe('NetworkPageStatusPanel when the network status request fails', () => {
  it('reports the error instead of an empty client list', async () => {
    server.use(
      http.get(API_ROUTES.network.status, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('No clients connected')).not.toBeInTheDocument();
  });

  it('shows the empty state only after a successful query with no clients', async () => {
    server.use(
      http.get(API_ROUTES.network.status, () =>
        HttpResponse.json({ ...mockNetworkStatus, clients: [] }),
      ),
    );
    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('No clients connected')).toBeInTheDocument();
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
