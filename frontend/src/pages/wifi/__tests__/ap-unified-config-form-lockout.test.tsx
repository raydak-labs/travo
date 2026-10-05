import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { API_ROUTES, type APConfig, type APConfigUpdate } from '@shared/index';
import { server } from '@/mocks/server';
import { mockAPConfigs } from '@/mocks/data';
import { WIFI_LOCKOUT_ERROR_CODE } from '@/lib/wifi-lockout';
import { APUnifiedConfigForm } from '../ap-unified-config-form';

interface PutCall {
  section: string;
  body: APConfigUpdate;
}

const PREVIOUS_SSID = mockAPConfigs[0]!.ssid;
const RENAME = 'Holiday';

/**
 * Records every AP PUT and answers it with `decide`, so the test states what
 * the DEVICE does rather than what the form happens to send. `index` counts
 * every request for the section, so a test can fail the *restore* of a PUT and
 * not just the apply.
 */
function onApPut(
  sent: PutCall[],
  decide: (section: string, body: APConfigUpdate, index: number) => Response,
) {
  const perSection = new Map<string, number>();
  server.use(
    http.put(`${API_ROUTES.wifi.ap}/:section`, async ({ params, request }) => {
      const section = params.section as string;
      const body = (await request.json()) as APConfigUpdate;
      const index = perSection.get(section) ?? 0;
      perSection.set(section, index + 1);
      sent.push({ section, body });
      return decide(section, body, index);
    }),
  );
}

/** A restore is identifiable by the device, not by the caller: it carries the old credentials. */
function isRestore(section: string, body: APConfigUpdate, target: APConfig) {
  return section === target.section && body.ssid === target.ssid && body.key === target.key;
}

const ok = () => HttpResponse.json({ status: 'ok' });

const refused = () =>
  HttpResponse.json(
    {
      error: 'connect over Ethernet first, or resend with acknowledge_lockout',
      code: WIFI_LOCKOUT_ERROR_CODE,
    },
    { status: 409 },
  );

function renderForm(
  enabledBySection: Record<string, boolean> = {},
  apConfigs: APConfig[] = mockAPConfigs,
) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnMount: false, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  });
  const onEnabledChange = vi.fn();
  const props = {
    enabledBySection,
    activeEnabledCount: Object.values(enabledBySection).filter(Boolean).length,
    onEnabledChange,
  };
  const view = render(
    <QueryClientProvider client={client}>
      <APUnifiedConfigForm apConfigs={apConfigs} {...props} />
    </QueryClientProvider>,
  );
  return {
    /** The parent refetching: the prop the form snapshots moves under it. */
    rerenderConfigs: (next: APConfig[]) =>
      view.rerender(
        <QueryClientProvider client={client}>
          <APUnifiedConfigForm apConfigs={next} {...props} />
        </QueryClientProvider>,
      ),
  };
}

async function renameAndSave(user: ReturnType<typeof userEvent.setup>, ssid = RENAME) {
  await user.clear(screen.getByPlaceholderText('SSID for all radios'));
  await user.type(screen.getByPlaceholderText('SSID for all radios'), ssid);
  await user.click(screen.getByRole('button', { name: 'Save' }));
}

async function acknowledge(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('checkbox'));
  await user.click(screen.getByRole('button', { name: /apply anyway/i }));
}

describe('APUnifiedConfigForm lockout guard', () => {
  it('re-sends every band with the acknowledgement only after the box is ticked', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (_section, body) => (body.acknowledge_lockout ? ok() : refused()));
    renderForm();

    await renameAndSave(user);

    expect(await screen.findByText(/this will disconnect you/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /apply anyway/i })).toBeDisabled();
    expect(sent).toHaveLength(1);
    expect(sent[0]!.section).toBe('default_radio0');
    expect(sent[0]!.body.acknowledge_lockout).toBeUndefined();

    await acknowledge(user);

    // One acknowledgement for the whole pending apply: every band is re-sent,
    // and each carries the same credentials the refused request did.
    await waitFor(() => expect(sent).toHaveLength(3));
    expect(sent.slice(1).map((p) => p.section)).toEqual(['default_radio0', 'default_radio1']);
    for (const call of sent.slice(1)) {
      expect(call.body.acknowledge_lockout).toBe(true);
      expect(call.body.ssid).toBe('Holiday');
      expect(call.body.key).toBe('travel12345');
      expect(call.body.enabled).toBe(true);
    }
    await waitFor(() =>
      expect(screen.queryByText(/this will disconnect you/i)).not.toBeInTheDocument(),
    );
  });

  it('sends nothing further when the operator cancels', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (_section, body) => (body.acknowledge_lockout ? ok() : refused()));
    renderForm();

    await renameAndSave(user);
    await screen.findByText(/this will disconnect you/i);
    await acknowledgeTickOnly(user);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/this will disconnect you/i)).not.toBeInTheDocument(),
    );
    expect(sent).toHaveLength(1);
  });

  // A refusal part-way through leaves the earlier bands already renamed. The
  // form puts them back first, so the pending change is still the whole apply
  // and one acknowledgement has to re-send every band, not just the refused
  // one.
  it('rolls the earlier band back and re-sends the whole apply once acknowledged', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (section, body) => {
      if (section === 'default_radio1' && !body.acknowledge_lockout) return refused();
      return ok();
    });
    renderForm({ default_radio0: false, default_radio1: true });

    await renameAndSave(user);

    expect(await screen.findByText(/this will disconnect you/i)).toBeInTheDocument();
    await waitFor(() => expect(sent).toHaveLength(3));
    expect(sent.map((p) => `${p.section}:${p.body.enabled}:${p.body.ssid}`)).toEqual([
      'default_radio0:false:Holiday',
      'default_radio1:true:Holiday',
      'default_radio0:true:OpenWrt-Travel',
    ]);

    await acknowledge(user);

    await waitFor(() => expect(sent).toHaveLength(5));
    expect(sent.slice(3).map((p) => p.section)).toEqual(['default_radio0', 'default_radio1']);
    expect(sent.slice(3).every((p) => p.body.acknowledge_lockout === true)).toBe(true);
  });

  it('does not show the dialog for a failure that is not the lockout refusal', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, () => HttpResponse.json({ error: 'uci commit failed' }, { status: 400 }));
    renderForm();

    await renameAndSave(user);

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to save');
    expect(screen.queryByText(/this will disconnect you/i)).not.toBeInTheDocument();
    expect(sent).toHaveLength(1);
  });
});

/** Tick the box but stop short of Apply: the ticked state alone sends nothing. */
async function acknowledgeTickOnly(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('checkbox'));
}

// The dialog's "Nothing has been changed yet" is a claim about the ROUTER, not
// about the request: it may only appear when every band the apply already wrote
// has been put back. These pin that precondition from both sides.
describe('APUnifiedConfigForm lockout dialog precondition', () => {
  it('shows the dialog when the rollback of the written bands fully succeeded', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (section, body) => {
      if (section === 'default_radio1' && !body.acknowledge_lockout) return refused();
      return ok();
    });
    renderForm({ default_radio0: false, default_radio1: true });

    await renameAndSave(user);

    expect(await screen.findByText(/this will disconnect you/i)).toBeInTheDocument();
    expect(sent.filter((p) => isRestore(p.section, p.body, mockAPConfigs[0]!))).toHaveLength(1);
  });

  // If the restore failed too, the router is holding a half-applied config. A
  // dialog promising "nothing has been changed yet" would be the one thing that
  // stops the operator looking for the section that did change.
  it('withholds the dialog when a rollback also failed, and names that section', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (section, body) => {
      if (isRestore(section, body, mockAPConfigs[0]!)) {
        return HttpResponse.json({ error: 'device busy' }, { status: 500 });
      }
      if (section === 'default_radio1' && !body.acknowledge_lockout) return refused();
      return ok();
    });
    renderForm({ default_radio0: false, default_radio1: true });

    await renameAndSave(user);

    // Assert the dialog is WITHHELD first. Under the failure mode this test
    // exists for, Radix's modal aria-hides the rest of the tree, so a
    // findByRole('alert') on its own can resolve to the dialog's own alert and
    // the two assertions that state the property below would never run.
    expect(screen.queryByRole('button', { name: /apply anyway/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/this will disconnect you/i)).not.toBeInTheDocument();

    const report = await screen.findByRole('alert');
    expect(report).toHaveTextContent('default_radio0');
    expect(report).toHaveTextContent(/still has the new settings/i);
    // And nothing offers to re-send: the router state has to be settled first.
    expect(screen.queryByRole('button', { name: /apply anyway/i })).not.toBeInTheDocument();
  });

  // Nothing written, nothing to put back: the router does hold its previous
  // settings, so the dialog is truthful here and the refusal is still a lockout.
  it('shows the dialog when the first band itself is refused, nothing to roll back', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, () => refused());
    renderForm();

    await renameAndSave(user);

    expect(await screen.findByText(/this will disconnect you/i)).toBeInTheDocument();
    expect(sent).toHaveLength(1);
    // A restore with nothing written would be a PUT nobody asked for.
    expect(sent.some((p) => isRestore(p.section, p.body, mockAPConfigs[0]!))).toBe(false);
    await acknowledge(user);
    await waitFor(() => expect(sent).toHaveLength(2));
  });
});

// A mid-apply refetch (useSetAPConfig invalidates ['wifi','ap'] after every
// success) can hand the form a prop where an earlier band already carries the
// NEW name. The acknowledged re-send must still roll back to what the router
// held before the FIRST attempt, not to whatever the prop says by then.
describe('APUnifiedConfigForm acknowledged re-send baseline', () => {
  it('rolls the re-send back to the pre-apply settings, not the refetched ones', async () => {
    const user = userEvent.setup();
    const sent: PutCall[] = [];
    onApPut(sent, (section) => {
      // Refuse the acknowledged apply on the last band too, so the re-send has
      // to roll back and the baseline it rolls back to becomes observable.
      if (section === 'default_radio1') return refused();
      return ok();
    });
    const { rerenderConfigs } = renderForm({ default_radio0: false, default_radio1: true });

    await renameAndSave(user);
    await screen.findByText(/this will disconnect you/i);
    await waitFor(() => expect(sent).toHaveLength(3));

    // The refetch lands while the operator reads the dialog: radio0 shows the
    // new name because the device really does hold it right now.
    rerenderConfigs(
      mockAPConfigs.map((ap) => (ap.section === 'default_radio0' ? { ...ap, ssid: RENAME } : ap)),
    );

    await acknowledge(user);

    // First apply: radio0 written, radio1 refused, radio0 restored (3 PUTs).
    // Re-send: same shape, because the acknowledged apply is refused on radio1
    // as well, so it rolls back too (3 more).
    await waitFor(() => expect(sent).toHaveLength(6));
    // The re-send's own restore puts back OpenWrt-Travel. Restoring RENAME here
    // would be a no-op that leaves the router permanently renamed.
    expect(sent.at(-1)).toMatchObject({ section: 'default_radio0', body: { ssid: PREVIOUS_SSID } });
  });
});
