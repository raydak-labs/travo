import { afterEach, describe, expect, it } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES, type Alert } from '@shared/index';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { useAlertStore } from '@/stores/alert-store';
import { server } from '@/mocks/server';
import { FailoverCard } from '../failover-card';

// The alert FailoverService.adoptSection publishes, in the words it publishes
// it: the section name is only ever inside the message, so the card has to
// lift it out of there.
const SECTION = 'travo_if_wwan';
const OVERWRITE_MESSAGE =
  `Failover adopted mwan3 section ${SECTION}: options Travo owns were replaced by the ` +
  'saved failover settings, other options were kept. Re-apply your tuning there if it was lost.';

function operatorEditAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-operator-edit',
    type: 'failover_operator_edits_overwritten',
    message: OVERWRITE_MESSAGE,
    severity: 'warning',
    timestamp: Date.now(),
    ...overrides,
  };
}

function publishAlerts(alerts: Alert[]) {
  act(() => {
    useAlertStore.setState({ alerts, unreadCount: alerts.length });
  });
}

function renderCard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <FailoverCard />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

async function saveOnce(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: 'Edit Failover Settings' }));
  await user.click(screen.getByRole('button', { name: 'Save Failover Settings' }));
  await screen.findByRole('button', { name: 'Edit Failover Settings' });
}

afterEach(() => {
  useAlertStore.setState({ alerts: [], unreadCount: 0 });
  window.localStorage.clear();
});

// A save that adopted a hand-edited section is the moment the operator's
// configuration changes under them; the card they pressed Save on has to say so.
describe('FailoverCard after a save that adopted an operator-edited mwan3 section', () => {
  it('warns inline, naming the section and what Travo did to it', async () => {
    server.use(http.put(API_ROUTES.network.failover, () => HttpResponse.json({ status: 'ok' })));
    const user = userEvent.setup();
    renderCard();

    await saveOnce(user);
    publishAlerts([operatorEditAlert()]);

    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent(SECTION);
    expect(warning).toHaveTextContent(/replaced/i);
    expect(warning).toHaveTextContent(/kept/i);
    // Acknowledging is not the only way out: the operator is told where to look.
    expect(warning).toHaveTextContent(new RegExp(`uci show mwan3\\.${SECTION}`));
  });

  it('keys off the alert type, not off the wording of one message', async () => {
    renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert({ message: 'A section you edited was taken over.' })]);

    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent(/mwan3 section you had edited/i);
    expect(warning).toHaveTextContent(/kept/i);
  });
  it('shows the warning in both view and edit mode so it cannot be edited past', async () => {
    server.use(http.put(API_ROUTES.network.failover, () => HttpResponse.json({ status: 'ok' })));
    const user = userEvent.setup();
    renderCard();

    await saveOnce(user);
    publishAlerts([operatorEditAlert()]);
    await screen.findByRole('alert');

    await user.click(screen.getByRole('button', { name: 'Edit Failover Settings' }));
    expect(screen.getByRole('alert')).toHaveTextContent(SECTION);
  });
});

// A banner on every load is noise, and noise is what gets ignored.
describe('FailoverCard when the save adopted nothing the operator had edited', () => {
  it('shows no overwrite warning, whatever else the alert feed carries', async () => {
    server.use(http.put(API_ROUTES.network.failover, () => HttpResponse.json({ status: 'ok' })));
    const user = userEvent.setup();
    renderCard();

    await saveOnce(user);
    publishAlerts([
      {
        id: 'alert-cpu',
        type: 'system_cpu_high',
        message: 'CPU usage above 90%',
        severity: 'warning',
        timestamp: Date.now(),
      },
      {
        id: 'alert-failover-saved',
        type: 'failover_updated',
        message: 'Failover settings saved',
        severity: 'info',
        timestamp: Date.now(),
      },
    ]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Edit Failover Settings' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByText(new RegExp(SECTION))).not.toBeInTheDocument();
  });

  // Durable, not a toast: the alert record is still in the feed after the
  // card is reloaded, so the operator is told again rather than once.
  it('still warns after a reload, with no save in the new session', async () => {
    renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert()]);

    expect(await screen.findByRole('alert')).toHaveTextContent(SECTION);
  });
});

// The feed is a ring of the last 50 alerts with no TTL, so an alert from days
// ago is still in it on every load. Describing it in the present tense tells
// the operator their settings were replaced a moment ago when nothing was
// saved at all, and pushes them to re-apply tuning that may already be right.
describe('FailoverCard overwrite warning age', () => {
  it('does not claim "you just saved" for an alert from three days ago', async () => {
    const threeDaysAgo = Date.now() - 3 * 24 * 60 * 60 * 1000;
    renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert({ timestamp: threeDaysAgo })]);

    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent(SECTION);
    expect(warning).not.toHaveTextContent(/just saved/i);
    // What is actually true: the takeover happened, at a stated time.
    expect(warning).toHaveTextContent(new Date(threeDaysAgo).toLocaleString());
  });

  it('still says "you just saved" for an alert from this minute', async () => {
    renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert()]);

    expect(await screen.findByRole('alert')).toHaveTextContent(/you just saved/i);
  });

  // A missing or nonsensical timestamp must not buy the present tense.
  it.each([
    ['zero', 0],
    ['not a number', Number.NaN],
    ['seconds, not millis', 1_700_000_000],
  ])('does not claim "you just saved" for an alert whose timestamp is %s', async (_label, ts) => {
    renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert({ timestamp: ts })]);

    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent(SECTION);
    expect(warning).not.toHaveTextContent(/just saved/i);
  });
});

// A refused save changed nothing, so there is nothing to warn about: the card
// renders what the feed reports, and a refusal leaves it silent.
describe('FailoverCard when the save was refused or the query failed', () => {
  it('does not warn about a refused save', async () => {
    server.use(
      http.put(API_ROUTES.network.failover, () =>
        HttpResponse.json({ error: 'apply failed' }, { status: 500 }),
      ),
    );
    const user = userEvent.setup();
    renderCard();

    await user.click(await screen.findByRole('button', { name: 'Edit Failover Settings' }));
    await user.click(screen.getByRole('button', { name: 'Save Failover Settings' }));

    // Still in the editor, because the save did not succeed.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Failover Settings' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not warn when the failover query itself fails', async () => {
    server.use(
      http.get(API_ROUTES.network.failover, () =>
        HttpResponse.json({ error: 'boom' }, { status: 500 }),
      ),
    );
    renderCard();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('boom');
    });
    publishAlerts([operatorEditAlert()]);

    // The card cannot know what a save overwrote if it cannot read the config.
    expect(screen.queryByText(new RegExp(SECTION))).not.toBeInTheDocument();
  });
});

describe('FailoverCard when the overwrite warning is dismissed', () => {
  it('stops showing it and leaves the card usable', async () => {
    server.use(http.put(API_ROUTES.network.failover, () => HttpResponse.json({ status: 'ok' })));
    const user = userEvent.setup();
    renderCard();

    await saveOnce(user);
    publishAlerts([operatorEditAlert()]);
    await screen.findByRole('alert');

    await user.click(screen.getByRole('button', { name: /dismiss/i }));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    // Dismissing hides the warning, not the card.
    expect(screen.getByText('Connection Failover')).toBeInTheDocument();
    const edit = screen.getByRole('button', { name: 'Edit Failover Settings' });
    expect(edit).toBeEnabled();
    await user.click(edit);
    expect(screen.getByRole('button', { name: 'Save Failover Settings' })).toBeInTheDocument();
    // And it stays dismissed while the operator works in the editor.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  // The alert feed is server-retained (last 50, no TTL), so the banner is still
  // in it on every visit. A dismissal held only in component state is lost the
  // moment the operator navigates away, and they are then shown — and must
  // clear — the same warning again on the next page load.
  it('stays dismissed when the operator navigates away and back', async () => {
    const user = userEvent.setup();
    const visit = renderCard();
    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    publishAlerts([operatorEditAlert()]);
    await screen.findByRole('alert');

    await user.click(screen.getByRole('button', { name: /dismiss/i }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    visit.unmount(); // the operator leaves the network page
    renderCard(); // and comes back to it

    await screen.findByRole('button', { name: 'Edit Failover Settings' });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  // Sticking dismissal must not become blanket suppression: the next save that
  // adopts a section is a different alert and has to be reported.
  it('still reports a later overwrite the operator has not dismissed', async () => {
    const user = userEvent.setup();
    publishAlerts([operatorEditAlert()]);
    const visit = renderCard();
    await screen.findByRole('alert');
    await user.click(screen.getByRole('button', { name: /dismiss/i }));

    publishAlerts([
      operatorEditAlert({
        id: 'alert-operator-edit-2',
        message: OVERWRITE_MESSAGE.replace(SECTION, 'travo_if_wan2'),
      }),
    ]);

    visit.unmount();
    renderCard();
    expect(await screen.findByRole('alert')).toHaveTextContent('travo_if_wan2');
  });
});
