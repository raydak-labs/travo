import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchTailscale } from '../api/tailscale';
import { fetchVpnStatus } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import VPN from './VPN.svelte';

const ready: LoadStatus = { state: 'ready' };
const { createLoader } = vi.hoisted(() => ({ createLoader: vi.fn() }));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
// The traffic rules have their own tests.
vi.mock('./VPNKillSwitchRow.svelte', () => ({ default: () => {} }));
vi.mock('./VPNSplitRow.svelte', () => ({ default: () => {} }));
vi.mock('./VPNDNSRow.svelte', () => ({ default: () => {} }));

const loader = (data: unknown) => ({ status: ready, data, retry: vi.fn(), stop: vi.fn() });

function page(profiles: unknown, tunnel: string, tailscale: unknown) {
  createLoader.mockImplementation((fn: unknown) =>
    fn === fetchVpnStatus
      ? loader({ state: tunnel, stale: false, observed_at: '', handshake_age_seconds: null })
      : fn === fetchTailscale
        ? loader(tailscale)
        : loader(profiles),
  );
  return render(VPN).body;
}

describe('VPN overview', () => {
  beforeEach(() => vi.clearAllMocks());

  it('links each VPN to its page and says why WireGuard cannot start', () => {
    const body = page({ profiles: [], selected_id: null }, 'disabled', { installed: false });
    expect(body).toContain('href="#/vpn/wireguard"');
    expect(body).toContain('href="#/vpn/tailscale"');
    expect(body).toContain('No profile yet');
    expect(switchState(body, 'vpn-tunnel-switch')).toEqual({ checked: false, disabled: true });
    expect(body).toContain('Not installed');
  });

  it('turns WireGuard on from the switch once a profile is chosen', () => {
    const body = page(
      { profiles: [{ id: 'p1', name: 'Mullvad DE' }], selected_id: 'p1' },
      'connected',
      { installed: true, running: true, backend_state: 'Running' },
    );
    expect(body).toContain('Mullvad DE');
    expect(switchState(body, 'vpn-tunnel-switch')).toEqual({ checked: true, disabled: false });
    expect(body).toContain('Connected');
  });
});
