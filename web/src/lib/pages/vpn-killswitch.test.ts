import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { KillSwitchState, TunnelStatus } from '../api/gen';
import { fetchKillSwitch } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import { killSwitchRefusal } from '../vpn-tunnel.svelte';
import VPNKillSwitchRow from './VPNKillSwitchRow.svelte';

const { createLoader, kill } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  kill: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as KillSwitchState | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const tunnel = (state: TunnelStatus['state'], stale = false): TunnelStatus => ({
  state,
  stale,
  observed_at: '2026-11-13T22:47:16Z',
  handshake_age_seconds: null,
});

function card(data: KillSwitchState | undefined, status?: TunnelStatus) {
  kill.data = data;
  return render(VPNKillSwitchRow, { props: { tunnel: status } }).body.replace(/<!--.*?-->/g, '');
}

const kill_ = (body: string) => switchState(body, 'vpn-kill-switch');

describe('VPN kill switch row', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockReturnValue(kill);
  });

  it('polls the kill switch state', () => {
    card({ enabled: false, zone: false }, tunnel('connected'));
    expect(createLoader).toHaveBeenCalledWith(fetchKillSwitch, 10000);
  });

  it('can be switched on only with a connected, fresh tunnel', () => {
    expect(kill_(card({ enabled: false, zone: false }, tunnel('connected')))).toEqual({
      checked: false,
      disabled: false,
    });
    for (const status of [
      tunnel('no_handshake'),
      tunnel('disabled'),
      tunnel('connected', true),
      undefined,
    ]) {
      const body = card({ enabled: false, zone: true }, status);
      expect(kill_(body)).toEqual({ checked: false, disabled: true });
      expect(body).toContain('Connect the tunnel');
    }
  });

  it('can always be switched off when on and explains the effect', () => {
    const body = card({ enabled: true, zone: true }, tunnel('disabled'));
    expect(kill_(body)).toEqual({ checked: true, disabled: false });
    expect(body).toContain('only through the tunnel');
  });

  it('maps refusals', () => {
    expect(killSwitchRefusal('vpn_not_connected').error).toBe(
      'Connect the tunnel before enabling the kill switch.',
    );
    expect(killSwitchRefusal('vpn_no_change').message).toBe(
      'The kill switch is already in that state.',
    );
    expect(killSwitchRefusal('vpn_killswitch_unavailable').error).toBe(
      'Could not change the kill switch.',
    );
  });
});
