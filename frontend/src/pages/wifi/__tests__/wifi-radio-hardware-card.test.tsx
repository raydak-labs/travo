import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { API_ROUTES } from '@shared/index';
import { server } from '@/mocks/server';
import { mockRadios } from '@/mocks/data';
import { WifiRadioHardwareCard } from '../wifi-radio-hardware-card';

// Radix Select drives pointer capture and scrolls the highlighted item into
// view; jsdom implements neither, so the dropdown cannot be opened without them.
Element.prototype.hasPointerCapture = () => false;
Element.prototype.releasePointerCapture = () => {};
Element.prototype.setPointerCapture = () => {};
Element.prototype.scrollIntoView = () => {};

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

function renderCard() {
  return render(<WifiRadioHardwareCard />, { wrapper });
}

/** Opens the role selector of the radio at the given index. */
async function openRoleSelector(user: ReturnType<typeof userEvent.setup>, index = 0) {
  const trigger = (await screen.findAllByRole('combobox'))[index];
  await waitFor(() => expect(trigger).not.toBeDisabled());
  await user.click(trigger);
  return screen.findByRole('option', { name: 'Both (repeater)' });
}

beforeEach(() => {
  localStorage.setItem('openwrt-auth-token', 'test-token');
});

describe('WifiRadioHardwareCard', () => {
  // The backend refuses role "both" on multi-radio hardware while
  // allow_ap_on_sta_radio is off. Offering it anyway meant the operator picked it
  // and got an error for a choice the UI could have refused up front.
  it('disables Both (repeater) and explains why on multi-radio hardware', async () => {
    const user = userEvent.setup();
    renderCard();

    const option = await openRoleSelector(user);

    await waitFor(() => expect(option).toHaveAttribute('aria-disabled', 'true'));
    expect(screen.getByText(/Both \(repeater\) is unavailable\./i)).toBeInTheDocument();
    // Both remedies, not just a greyed-out option.
    expect(screen.getByText(/own radio/i)).toBeInTheDocument();
    // Names the reachable control, not a UCI option and not a wizard that
    // nothing mounts.
    expect(
      screen.getByText(/Allow Wi-Fi on uplink radio.*Wi-Fi > Advanced > Repeater/i),
    ).toBeInTheDocument();
  });

  it('keeps Both (repeater) selectable when allow_ap_on_sta_radio is set', async () => {
    server.use(
      http.get(API_ROUTES.wifi.repeaterOptions, () =>
        HttpResponse.json({ allow_ap_on_sta_radio: true }),
      ),
    );
    const user = userEvent.setup();
    renderCard();

    const option = await openRoleSelector(user);

    await waitFor(() => expect(option).not.toHaveAttribute('aria-disabled', 'true'));
    expect(screen.queryByText(/Both \(repeater\) is unavailable\./i)).not.toBeInTheDocument();
  });

  // Single-radio hardware has no split to make, so the backend honours "both".
  it('keeps Both (repeater) selectable on single-radio hardware', async () => {
    server.use(http.get(API_ROUTES.wifi.radios, () => HttpResponse.json([mockRadios[0]])));
    const user = userEvent.setup();
    renderCard();

    const option = await openRoleSelector(user);

    await waitFor(() => expect(option).not.toHaveAttribute('aria-disabled', 'true'));
  });

  // The refusal message names both remedies; a toast alone disappears before an
  // operator has read it, so it has to sit next to the selector.
  it('shows the backend refusal inline when a role change is rejected', async () => {
    const refusal =
      'refusing to run an access point and the WiFi uplink on the same radio: give ' +
      'the uplink STA its own radio and put the downlink access point on the other ' +
      'one, or enable allow_ap_on_sta_radio in repeater options first';
    server.use(
      http.put(`${API_ROUTES.wifi.radios}/:name/role`, () =>
        HttpResponse.json({ error: refusal }, { status: 409 }),
      ),
    );
    const user = userEvent.setup();
    renderCard();

    await openRoleSelector(user);
    await user.click(screen.getByRole('option', { name: 'AP only' }));
    // A role change is disruptive, so it no longer applies on selection.
    await user.click(await screen.findByRole('button', { name: /change role/i }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Radio role was not changed.');
    expect(alert).toHaveTextContent(refusal);
  });

  // generated_key is returned exactly once, in this response; the new AP on the
  // air already uses it.
  it('shows a generated passphrase inline after the service invents one', async () => {
    server.use(
      http.put(`${API_ROUTES.wifi.radios}/:name/role`, () =>
        HttpResponse.json({ status: 'ok', generated_key: 'inv3nted-passphrase' }),
      ),
    );
    const user = userEvent.setup();
    renderCard();

    await openRoleSelector(user);
    await user.click(screen.getByRole('option', { name: 'AP only' }));
    await user.click(await screen.findByRole('button', { name: /change role/i }));

    const status = await screen.findByRole('status');
    expect(status).toHaveTextContent('inv3nted-passphrase');
    expect(status).toHaveTextContent('radio0');
  });
});

describe('WifiRadioHardwareCard lockout guard', () => {
  beforeEach(() => {
    localStorage.setItem('openwrt-auth-token', 'test-token');
  });

  // Switching the last access-point-carrying radio off while on WiFi is refused
  // by the router (ADR 0002 §5); the UI has to re-send it with the explicit
  // acknowledgement and send nothing at all if the operator backs out.
  it('re-sends a refused role change with acknowledge_lockout after the box is ticked', async () => {
    const user = userEvent.setup();
    const sent: { role: string; acknowledge_lockout?: boolean }[] = [];
    let refused = true;
    server.use(
      http.put(API_ROUTES.wifi.radioRole, async ({ request }) => {
        const body = (await request.json()) as { role: string; acknowledge_lockout?: boolean };
        sent.push(body);
        if (refused) {
          refused = false;
          return HttpResponse.json(
            {
              error: 'connect over Ethernet first',
              code: 'wifi_lockout_risk',
            },
            { status: 409 },
          );
        }
        return HttpResponse.json({ status: 'ok', apply: null });
      }),
      http.post(API_ROUTES.wifi.applyConfirm, () => HttpResponse.json({ status: 'ok' })),
    );
    renderCard();

    await openRoleSelector(user);
    await user.click(await screen.findByRole('option', { name: 'Disabled' }));
    await user.type(await screen.findByPlaceholderText('Type CONFIRM'), 'CONFIRM');
    const disable = await screen.findByRole('button', { name: /disable radio/i });
    await waitFor(() => expect(disable).not.toBeDisabled());
    await user.click(disable);

    expect(await screen.findByText(/This will disconnect you/i)).toBeInTheDocument();
    // The refusal came from a real request: it went out WITHOUT the
    // acknowledgement and the router said no. Nothing is re-sent until the box
    // is ticked.
    expect(sent).toEqual([{ role: 'none' }]);
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: /apply anyway/i }));

    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]).toEqual({ role: 'none', acknowledge_lockout: true });
  });

  it('sends nothing when the acknowledgement dialog is dismissed', async () => {
    const user = userEvent.setup();
    const sent: { role: string; acknowledge_lockout?: boolean }[] = [];
    server.use(
      http.put(API_ROUTES.wifi.radioRole, async ({ request }) => {
        sent.push((await request.json()) as { role: string; acknowledge_lockout?: boolean });
        return HttpResponse.json(
          { error: 'connect over Ethernet first', code: 'wifi_lockout_risk' },
          { status: 409 },
        );
      }),
    );
    renderCard();

    await openRoleSelector(user);
    await user.click(await screen.findByRole('option', { name: 'Disabled' }));
    await user.type(await screen.findByPlaceholderText('Type CONFIRM'), 'CONFIRM');
    const disable = await screen.findByRole('button', { name: /disable radio/i });
    await waitFor(() => expect(disable).not.toBeDisabled());
    await user.click(disable);

    await screen.findByText(/This will disconnect you/i);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/This will disconnect you/i)).not.toBeInTheDocument(),
    );
    expect(sent).toHaveLength(1);
    expect(sent[0]!.acknowledge_lockout).toBeUndefined();
  });
});
