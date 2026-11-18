import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { TunnelStatus } from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';

const { createLoader, dashboardEvents, vpn } = vi.hoisted(() => {
  const idle = () => ({ status: { state: 'ready' }, data: undefined, retry: () => {} });
  return {
    createLoader: vi.fn(),
    dashboardEvents: {
      connectivity: idle(),
      resources: idle(),
      throughput: idle(),
      tailscale: idle(),
    },
    vpn: {
      status: { state: 'ready' } as LoadStatus,
      data: undefined as TunnelStatus | undefined,
      retry: () => {},
      stop: () => {},
    },
  };
});
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

function dashboard(
  state: TunnelStatus['state'] | undefined,
  status: LoadStatus = { state: 'ready' },
) {
  vpn.status = status;
  vpn.data = state && {
    state,
    stale: false,
    observed_at: '2026-11-13T22:47:16Z',
    handshake_age_seconds: null,
  };
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body.replace(
    /<!--.*?-->/g,
    '',
  );
}

const button = (label: string) => new RegExp(`<button[^>]*>${label}</button>`);
const disabledButton = (label: string) =>
  new RegExp(`<button[^>]*disabled=""[^>]*>${label}</button>`);

describe('Dashboard VPN quick action', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
    createLoader.mockImplementation((fn: unknown) =>
      fn === fetchVpnStatus
        ? vpn
        : { status: { state: 'ready' }, data: undefined, retry: () => {}, stop: () => {} },
    );
  });
  afterEach(() => vi.unstubAllGlobals());

  it('polls the same status as the VPN page', () => {
    dashboard('configured');
    expect(createLoader).toHaveBeenCalledWith(fetchVpnStatus, 10000);
  });

  it('offers Enable when a profile is ready', () => {
    const body = dashboard('configured');
    expect(body).toContain('Profile ready');
    expect(body).toMatch(button('Enable tunnel'));
    expect(body).not.toMatch(disabledButton('Enable tunnel'));
  });

  it('disables Enable without a selected profile and links to VPN settings', () => {
    const body = dashboard('disabled');
    expect(body).toMatch(disabledButton('Enable tunnel'));
    expect(body).toMatch(/<a[^>]*href="#\/vpn"[^>]*>VPN settings<\/a>/);
  });

  it.each(['enabled_not_up', 'no_handshake', 'connected'] as const)(
    'offers Disable when %s',
    (state) => {
      const body = dashboard(state);
      expect(body).toMatch(button('Disable tunnel'));
      expect(body).not.toContain('Apply selected profile');
    },
  );

  it('shows unavailable status without a toggle', () => {
    const body = dashboard(undefined, { state: 'error', message: 'Tunnel status unavailable' });
    expect(body).toContain('Tunnel status unavailable');
    expect(body).not.toContain('Enable tunnel');
    expect(body).not.toContain('Disable tunnel');
  });
});
