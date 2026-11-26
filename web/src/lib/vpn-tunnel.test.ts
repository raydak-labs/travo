import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { client } from './api/gen/client.gen';
import { createTunnelControl, tunnelEnabled, tunnelRefusal } from './vpn-tunnel.svelte';

// The same daemon contract the VPN page uses: POST /vpn/tunnel, then the transaction routes.
const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

const txn = (state: string) => ({
  configs: ['network'],
  id: 'tx1',
  started: '2026-11-13T22:47:16Z',
  state,
  summary: 'Enable WireGuard tunnel',
  window_seconds: 120,
});

type Route = Record<string, () => Response | Promise<Response>>;
function serve(routes: Route) {
  const calls: string[] = [];
  const fetchFn = vi.fn(async (request: Request) => {
    const key = `${request.method} ${new URL(request.url).pathname}`;
    calls.push(key);
    const hit = routes[key];
    if (!hit) throw new Error(`unexpected ${key}`);
    return hit();
  });
  client.setConfig({ baseUrl: 'http://daemon.test', fetch: fetchFn as unknown as typeof fetch });
  return calls;
}

const TUNNEL = 'POST /api/v1/vpn/tunnel';
let control: ReturnType<typeof createTunnelControl>;
let refresh: ReturnType<typeof vi.fn<() => void>>;

beforeEach(() => {
  refresh = vi.fn<() => void>();
  control = createTunnelControl(refresh);
});
afterEach(() => control.stop());

describe('tunnel state helpers', () => {
  it.each([
    ['disabled', false],
    ['configured', false],
    ['enabled_not_up', true],
    ['no_handshake', true],
    ['connected', true],
  ] as const)('%s enabled=%s', (state, enabled) => {
    expect(tunnelEnabled(state)).toBe(enabled);
  });

  it('maps refusal codes', () => {
    expect(tunnelRefusal('vpn_profile_required').error).toBe('Select a WireGuard profile first.');
    expect(tunnelRefusal('vpn_no_change').message).toBe('Tunnel settings are already up to date.');
    expect(tunnelRefusal('transaction_pending').error).toBe(
      'Another configuration change is pending.',
    );
    expect(tunnelRefusal('vpn_tunnel_unavailable').error).toBe(
      'Could not apply the tunnel change.',
    );
  });
});

describe('createTunnelControl', () => {
  it('enables, waits for confirmation, then refreshes status', async () => {
    const calls = serve({
      [TUNNEL]: () => json(202, txn('pending')),
      'POST /api/v1/transactions/tx1/confirm': () => json(200, txn('confirmed')),
    });
    await control.set(true);
    expect(control.transaction?.state).toBe('pending');
    // The toast counts down the 120 s window from when the transaction arrived.
    expect(control.deadline).toBeGreaterThan(performance.now() + 119_000);
    expect(control.busy).toBe(true);
    expect(refresh).not.toHaveBeenCalled();
    await control.settle(true);
    expect(control.message).toBe('Tunnel change confirmed.');
    expect(control.busy).toBe(false);
    expect(refresh).toHaveBeenCalledOnce();
    expect(calls).toEqual([TUNNEL, 'POST /api/v1/transactions/tx1/confirm']);
  });

  it('disables and rolls back on request', async () => {
    serve({
      [TUNNEL]: () => json(202, { ...txn('pending'), summary: 'Disable WireGuard tunnel' }),
      'POST /api/v1/transactions/tx1/rollback': () => json(200, txn('rolled_back')),
    });
    await control.set(false);
    await control.settle(false);
    expect(control.message).toBe('Tunnel change rolled back.');
    expect(refresh).toHaveBeenCalledOnce();
  });

  it('ignores a duplicate submission while a request is in flight', async () => {
    let release: (r: Response) => void = () => {};
    const calls = serve({ [TUNNEL]: () => new Promise((resolve) => (release = resolve)) });
    const first = control.set(true);
    expect(control.applying).toBe(true);
    await control.set(true);
    await vi.waitFor(() => expect(calls).toEqual([TUNNEL]));
    release(json(202, txn('pending')));
    await first;
    expect(calls).toEqual([TUNNEL]);
    await control.set(false); // still pending: refused locally
    expect(calls).toEqual([TUNNEL]);
  });

  it('polls a pending transaction to its outcome', async () => {
    serve({
      [TUNNEL]: () => json(202, txn('pending')),
      'GET /api/v1/transactions/tx1': () => json(200, txn('rolled_back')),
    });
    await control.set(true);
    await control.poll();
    expect(control.transaction?.state).toBe('rolled_back');
    expect(refresh).toHaveBeenCalledOnce();
  });

  it('reports a lost transaction as unknown, not rolled back', async () => {
    serve({
      [TUNNEL]: () => json(202, txn('pending')),
      'GET /api/v1/transactions/tx1': () => json(404, { error: 'not found' }),
    });
    await control.set(true);
    await control.poll();
    expect(control.transaction).toBeUndefined();
    expect(control.error).toBe('The transaction result is unavailable. Check the tunnel status.');
  });

  it('shows the missing capability as a failure and refreshes status', async () => {
    serve({ [TUNNEL]: () => json(503, { error: 'unavailable', code: 'vpn_tunnel_unavailable' }) });
    await control.set(true);
    expect(control.error).toBe('Could not apply the tunnel change.');
    expect(control.busy).toBe(false);
    expect(refresh).toHaveBeenCalledOnce();
  });

  it('shows a network failure', async () => {
    serve({ [TUNNEL]: () => Promise.reject(new Error('offline')) });
    await control.set(true);
    expect(control.error).toBe('Could not apply the tunnel change.');
  });

  it('shows a failed restore', async () => {
    serve({
      [TUNNEL]: () => json(202, txn('pending')),
      'POST /api/v1/transactions/tx1/rollback': () =>
        json(409, { error: 'failed', code: 'rollback_failed' }),
    });
    await control.set(true);
    await control.settle(false);
    expect(control.error).toBe('Tunnel restore failed. Check the router before trying again.');
  });
});
