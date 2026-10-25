import { describe, expect, it } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { mockUpdateStatus } from '@/mocks/data';
import { SystemUpdatesCard } from '../system-updates-card';

function renderCard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SystemUpdatesCard />
    </QueryClientProvider>,
  );
}

describe('SystemUpdatesCard', () => {
  it('links a new Travo release to its notes', async () => {
    renderCard();
    expect(await screen.findByText('Travo 0.10.0 available')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /release notes/i })).toHaveAttribute(
      'href',
      'https://github.com/raydak-labs/travo/releases/tag/v0.10.0',
    );
  });

  it('sends a new OpenWrt release to LuCI', async () => {
    renderCard();
    expect(await screen.findByText('OpenWrt 23.05.6 available')).toBeInTheDocument();
    const link = screen.getByRole('link', { name: /upgrade in luci/i });
    expect(link).toHaveAttribute(
      'href',
      `http://${window.location.hostname}:8080/cgi-bin/luci/admin/system/attendedsysupgrade`,
    );
  });

  it('upgrades the packages after a confirmation and shows the log', async () => {
    const user = userEvent.setup();
    renderCard();
    await user.click(await screen.findByRole('button', { name: 'Upgrade all' }));

    const confirm = await screen.findByRole('dialog');
    expect(within(confirm).getByText(/WiFi clients and DNS may drop/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Upgrade' }));

    const log = await screen.findByRole('log');
    await waitFor(() => expect(log).toHaveTextContent('Upgrading dnsmasq (2.91-r2 -> 2.93-r1)'));
    expect(await screen.findByText('Operation completed successfully.')).toBeInTheDocument();
  });

  it('says so before the first check instead of claiming the router is current', async () => {
    server.use(
      http.get(API_ROUTES.system.updates, () =>
        HttpResponse.json({ ...mockUpdateStatus, checked_at: '' }),
      ),
    );
    renderCard();
    expect(await screen.findByText(/Not checked yet/)).toBeInTheDocument();
    expect(screen.queryByText(/newest release/)).not.toBeInTheDocument();
  });
});
