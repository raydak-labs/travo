import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { TunnelStatus } from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
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
      uplinks: idle(),
      clients: idle(),
      alerts: idle(),
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

  it('can switch the tunnel on when a profile is ready', () => {
    const body = dashboard('configured');
    expect(body).toContain('Profile ready');
    expect(switchState(body, 'dashboard-vpn-switch')).toEqual({ checked: false, disabled: false });
  });

  it('cannot switch on without a selected profile, says why and links to VPN', () => {
    const body = dashboard('disabled');
    expect(switchState(body, 'dashboard-vpn-switch')).toEqual({ checked: false, disabled: true });
    expect(body).toContain('No profile yet');
    expect(body).toContain('href="#/vpn"');
  });

  it.each(['enabled_not_up', 'no_handshake', 'connected'] as const)(
    'can switch the tunnel off when %s',
    (state) => {
      expect(switchState(dashboard(state), 'dashboard-vpn-switch')).toEqual({
        checked: true,
        disabled: false,
      });
    },
  );

  it('shows unavailable status without a toggle', () => {
    const body = dashboard(undefined, { state: 'error', message: 'Tunnel status unavailable' });
    expect(body).toContain('Tunnel status unavailable');
    expect(switchState(body, 'dashboard-vpn-switch')).toBeNull();
  });
});
