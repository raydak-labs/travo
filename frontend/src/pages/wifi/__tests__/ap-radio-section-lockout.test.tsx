import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw/http';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { APConfig, APConfigUpdate } from '@shared/index';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { WIFI_LOCKOUT_ERROR_CODE } from '@/lib/wifi-lockout';
import { APRadioSection } from '../ap-radio-section';

/** One enabled access point: disabling it is the lockout case. */
const onlyAp: APConfig = {
  radio: 'radio0',
  band: '2g',
  ssid: 'OpenWrt-Travel',
  encryption: 'psk2',
  key: 'travel12345',
  enabled: true,
  channel: 6,
  section: 'default_radio0',
};

function renderSection() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <APRadioSection ap={onlyAp} activeEnabledCount={1} onEnabledChange={() => {}} />
    </QueryClientProvider>,
  );
}

/** Refuses the first disable, accepts the acknowledged one. */
function refuseOnce(sent: APConfigUpdate[]) {
  let refused = false;
  server.use(
    http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ request }) => {
      sent.push((await request.json()) as APConfigUpdate);
      if (!refused) {
        refused = true;
        return HttpResponse.json(
          { error: 'connect over Ethernet first', code: WIFI_LOCKOUT_ERROR_CODE },
          { status: 409 },
        );
      }
      return HttpResponse.json({ status: 'ok', apply: null });
    }),
    http.post(API_ROUTES.wifi.applyConfirm, () => HttpResponse.json({ status: 'ok' })),
  );
}

async function disableAndConfirm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByLabelText('Enabled'));
  await user.click(await screen.findByRole('button', { name: 'Save' }));
  await user.click(await screen.findByRole('button', { name: 'Disable' }));
}

describe('APRadioSection lockout guard', () => {
  it('re-sends the disable with the acknowledgement only after the box is ticked', async () => {
    const user = userEvent.setup();
    const sent: APConfigUpdate[] = [];
    refuseOnce(sent);
    renderSection();

    await disableAndConfirm(user);

    expect(await screen.findByText(/This will disconnect you/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /apply anyway/i })).toBeDisabled();
    expect(sent).toHaveLength(1);
    expect(sent[0]!.enabled).toBe(false);
    expect(sent[0]!.acknowledge_lockout).toBeUndefined();

    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: /apply anyway/i }));

    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]!.enabled).toBe(false);
    expect(sent[1]!.acknowledge_lockout).toBe(true);
  });

  it('leaves the configuration alone when the operator cancels', async () => {
    const user = userEvent.setup();
    const sent: APConfigUpdate[] = [];
    refuseOnce(sent);
    renderSection();

    await disableAndConfirm(user);
    await screen.findByText(/This will disconnect you/i);
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/This will disconnect you/i)).not.toBeInTheDocument(),
    );
    expect(sent).toHaveLength(1);
  });
});
