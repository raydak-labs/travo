import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { server } from '@/mocks/server';
import { DhcpPoolSettingsCard } from '../dhcp-pool-settings-card';
import { LanDnsSettingsCard } from '../lan-dns-settings-card';

function renderCard(ui: React.ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>{ui}</ThemeProvider>
    </QueryClientProvider>,
  );
}

// A single failed GET must not leave an editable form showing values the app
// never read: a Save would PUT the hard-coded defaults over the real config.
describe('network config cards when the GET fails', () => {
  it('DhcpPoolSettingsCard shows an error, not the 100/150/12h defaults', async () => {
    server.use(
      http.get(API_ROUTES.network.dhcp, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderCard(<DhcpPoolSettingsCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });

    expect(screen.queryByRole('button', { name: /Save DHCP Settings/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
  });

  it('LanDnsSettingsCard shows an error, not an empty custom-DNS form', async () => {
    server.use(
      http.get(API_ROUTES.network.dns, () =>
        HttpResponse.json({ error: 'ubus busy' }, { status: 500 }),
      ),
    );
    renderCard(<LanDnsSettingsCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('ubus busy');
    });

    expect(screen.queryByRole('button', { name: /Save DNS Settings/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('re-reads the config after Retry and only then offers the form', async () => {
    const user = userEvent.setup();
    let failNext = true;
    server.use(
      http.get(API_ROUTES.network.dhcp, () => {
        if (failNext) {
          failNext = false;
          return HttpResponse.json({ error: 'ubus busy' }, { status: 500 });
        }
        return HttpResponse.json({ start: 42, limit: 80, lease_time: '6h' });
      }),
    );
    renderCard(<DhcpPoolSettingsCard />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: /Save DHCP Settings/ })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Save DHCP Settings/ })).toBeInTheDocument();
    });
    const [start, limit] = screen.getAllByRole('spinbutton') as HTMLInputElement[];
    expect(start).toHaveValue(42);
    expect(limit).toHaveValue(80);
  });
});
