import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';
import { WIFI_LOCKOUT_ERROR_CODE } from '@/lib/wifi-lockout';

type ModeRequest = { mode: string; acknowledge_lockout?: boolean };

function renderCard() {
  server.use(
    // The device is in repeater mode and the operator reaches it over WiFi: the
    // shape in which switching to Client removes the access point.
    http.get(API_ROUTES.wifi.connection, () =>
      HttpResponse.json({
        ssid: 'Hotel_Guest_5G',
        bssid: '00:11:22:33:44:55',
        mode: 'repeater',
        signal_dbm: -42,
        signal_percent: 82,
        channel: 36,
        encryption: 'wpa2',
        band: '5ghz',
        ip_address: '192.168.1.105',
        connected: true,
      }),
    ),
    http.get(API_ROUTES.network.connectionMethod, () =>
      HttpResponse.json({ method: 'wifi-client', interface: 'wwan0' }),
    ),
  );
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <WifiModeCard />
    </QueryClientProvider>,
  );
}

/** Answers the first mode request with the lockout refusal, the rest with ok. */
function refuseOnce(sent: ModeRequest[]) {
  let refused = false;
  server.use(
    http.put(API_ROUTES.wifi.mode, async ({ request }) => {
      const body = (await request.json()) as ModeRequest;
      sent.push(body);
      if (!refused) {
        refused = true;
        return HttpResponse.json(
          {
            error:
              'refusing to remove the access point you are connected through: connect over ' +
              'Ethernet first',
            code: WIFI_LOCKOUT_ERROR_CODE,
          },
          { status: 409 },
        );
      }
      return HttpResponse.json({ status: 'ok', apply: null });
    }),
    http.post(API_ROUTES.wifi.applyConfirm, () => HttpResponse.json({ status: 'ok' })),
  );
}

async function requestClientMode(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByText('Client (STA)');
  await user.click(screen.getByRole('button', { name: /Client \(STA\)/ }));
  await user.click(await screen.findByRole('button', { name: 'I understand, switch mode' }));
}

describe('WifiModeCard lockout guard', () => {
  it('asks for an explicit acknowledgement and then re-sends with it', async () => {
    const user = userEvent.setup();
    const sent: ModeRequest[] = [];
    refuseOnce(sent);
    renderCard();

    await requestClientMode(user);

    // The refusal is a dialog that cannot be accepted without ticking the box,
    // and nothing has been re-sent yet.
    const dialog = await screen.findByText(/This will disconnect you/i);
    expect(dialog).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /apply anyway/i })).toBeDisabled();
    expect(sent).toHaveLength(1);
    expect(sent[0]).toEqual({ mode: 'client' });

    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: /apply anyway/i }));

    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]).toEqual({ mode: 'client', acknowledge_lockout: true });
  });

  it('sends nothing further when the operator cancels the acknowledgement', async () => {
    const user = userEvent.setup();
    const sent: ModeRequest[] = [];
    refuseOnce(sent);
    renderCard();

    await requestClientMode(user);
    await screen.findByText(/This will disconnect you/i);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/This will disconnect you/i)).not.toBeInTheDocument(),
    );
    expect(sent).toHaveLength(1);
  });

  // The failure that must NOT be dressed up as a lockout: no acknowledgement
  // dialog, and in particular no second, acknowledged request.
  it('does not show the lockout dialog for an unrelated failure', async () => {
    const user = userEvent.setup();
    const sent: ModeRequest[] = [];
    server.use(
      http.put(API_ROUTES.wifi.mode, async ({ request }) => {
        sent.push((await request.json()) as ModeRequest);
        return HttpResponse.json({ error: 'rpcd is unavailable' }, { status: 500 });
      }),
    );
    renderCard();

    await requestClientMode(user);

    await waitFor(() => expect(sent).toHaveLength(1));
    expect(screen.queryByText(/This will disconnect you/i)).toBeNull();
    expect(screen.queryByRole('checkbox')).toBeNull();
  });

  it('sends the mode straight away when the router does not refuse it', async () => {
    const user = userEvent.setup();
    const sent: ModeRequest[] = [];
    server.use(
      http.put(API_ROUTES.wifi.mode, async ({ request }) => {
        sent.push((await request.json()) as ModeRequest);
        return HttpResponse.json({ status: 'ok', apply: null });
      }),
    );
    renderCard();

    await requestClientMode(user);

    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toEqual({ mode: 'client' });
    expect(screen.queryByText(/This will disconnect you/i)).toBeNull();
  });
});
