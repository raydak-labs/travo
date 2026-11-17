import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { TunnelStatus } from '../api/gen';
import { fetchKillSwitch, fetchVpnStatus } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import VPN from './VPN.svelte';

const { createLoader, kill, profiles, tunnel } = vi.hoisted(() => ({
  kill: { status: { state: 'loading' }, data: undefined, retry: vi.fn(), stop: vi.fn() },
  createLoader: vi.fn(),
  profiles: {
    status: { state: 'ready' },
    data: { profiles: [], selected_id: null },
    retry: vi.fn(),
    stop: vi.fn(),
  },
  tunnel: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as TunnelStatus | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

function vpn(status: TunnelStatus) {
  tunnel.data = status;
  return render(VPN).body;
}

function snapshot(state: TunnelStatus['state']): TunnelStatus {
  return {
    state,
    stale: false,
    observed_at: '2026-11-13T22:47:16Z',
    handshake_age_seconds: null,
  };
}

describe('VPN tunnel status', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    tunnel.status = { state: 'ready' };
    createLoader.mockImplementation((fn: unknown) =>
      fn === fetchVpnStatus ? tunnel : fn === fetchKillSwitch ? kill : profiles,
    );
  });

  it.each([
    ['disabled', 'Off'],
    ['configured', 'Profile ready'],
    ['enabled_not_up', 'Interface down'],
    ['no_handshake', 'No handshake'],
    ['connected', 'Connected'],
  ] as const)('renders %s as %s', (state, label) => {
    const body = vpn(snapshot(state));
    expect(body).toContain(label);
    expect(createLoader).toHaveBeenCalledWith(fetchVpnStatus, 10000);
  });

  it('renders handshake age', () => {
    const body = vpn({ ...snapshot('connected'), handshake_age_seconds: 14 });
    expect(body).toContain('Last handshake &lt;1m ago');
  });

  it('renders the stale badge with a last good value', () => {
    const body = vpn({ ...snapshot('connected'), stale: true });
    expect(body).toContain('Last known value');
    expect(body).toContain('Connected');
  });
});
