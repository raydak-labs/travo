import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { UptimeLogCard } from '../uptime-log-card';
import { InterfacesCard } from '../interfaces-card';
import { DhcpLeasesCard } from '../dhcp-leases-card';
import { WanStatusCard } from '../wan-status-card';
import { DoHCard } from '../doh-card';
import { DataUsageSection } from '../data-usage-section';

function renderCard(ui: React.ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>{ui}</ThemeProvider>
    </QueryClientProvider>,
  );
}

function fail(route: string) {
  server.use(http.get(route, () => HttpResponse.json({ error: 'ubus busy' }, { status: 500 })));
}

/**
 * Every one of these cards used to read an undefined payload as an "absent"
 * fact: no connectivity events, no interfaces, no leases, no uplink, DNS off,
 * vnstat not installed. On a flaky link each of those is a confident lie, so
 * the request has to fail loudly instead.
 */
describe('network cards when their query fails', () => {
  it('UptimeLogCard reports the error instead of "no events recorded"', async () => {
    fail(API_ROUTES.network.uptimeLog);
    renderCard(<UptimeLogCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('No connectivity events recorded yet')).not.toBeInTheDocument();
  });

  it('InterfacesCard reports the error instead of "no interfaces found"', async () => {
    fail(API_ROUTES.network.status);
    renderCard(<InterfacesCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('No interfaces found')).not.toBeInTheDocument();
  });

  it('DhcpLeasesCard reports the error instead of "no active leases"', async () => {
    fail(API_ROUTES.network.dhcpLeases);
    renderCard(<DhcpLeasesCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('No active leases')).not.toBeInTheDocument();
  });

  it('WanStatusCard reports the error instead of "no uplink"', async () => {
    fail(API_ROUTES.network.status);
    renderCard(<WanStatusCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('No Uplink')).not.toBeInTheDocument();
  });

  it('DoHCard reports the error instead of "Disabled"', async () => {
    fail(API_ROUTES.network.doh);
    renderCard(<DoHCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText('Disabled')).not.toBeInTheDocument();
  });

  it('DataUsageSection reports the error instead of "install vnstat"', async () => {
    fail(API_ROUTES.network.dataUsage);
    renderCard(<DataUsageSection />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });
    expect(screen.queryByText(/Install the Data Usage/)).not.toBeInTheDocument();
  });
});

describe('UptimeLogCard retry', () => {
  it('retries the request that failed', async () => {
    const user = userEvent.setup();
    fail(API_ROUTES.network.uptimeLog);
    renderCard(<UptimeLogCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const retried = vi.fn();
    server.events.removeAllListeners();
    server.use(
      http.get(API_ROUTES.network.uptimeLog, () => {
        retried();
        return HttpResponse.json([]);
      }),
    );

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => {
      expect(retried).toHaveBeenCalled();
    });
  });
});
