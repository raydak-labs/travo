import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { FailoverCard } from '../failover-card';
import { LanConfigCard } from '../lan-config-card';
import { WanConfigCard } from '../wan-config-card';

function renderCard(ui: React.ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>{ui}</ThemeProvider>
    </QueryClientProvider>,
  );
}

function failNetworkStatus() {
  server.use(
    http.get(API_ROUTES.network.status, () =>
      HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
    ),
  );
}

// Each of these used to render a confident "absent" claim from an undefined
// value after a single failed GET.
describe('network cards when GET /network/status fails', () => {
  it('WanConfigCard reports the error instead of "WAN not configured"', async () => {
    failNetworkStatus();
    renderCard(<WanConfigCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('WAN not configured')).not.toBeInTheDocument();
  });

  it('LanConfigCard reports the error', async () => {
    failNetworkStatus();
    renderCard(<LanConfigCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
  });
});

describe('FailoverCard when GET /network/failover fails', () => {
  it('reports the error instead of "not available"', async () => {
    server.use(
      http.get(API_ROUTES.network.failover, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderCard(<FailoverCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('Failover configuration is not available.')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Edit Failover Settings/ }),
    ).not.toBeInTheDocument();
  });
});
