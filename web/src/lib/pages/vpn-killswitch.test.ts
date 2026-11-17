import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { KillSwitchState, TunnelStatus } from '../api/gen';
import { fetchKillSwitch } from '../api/vpn-status';
import type { LoadStatus } from '../ui/loader.svelte';
import { killSwitchRefusal } from '../vpn-tunnel.svelte';
import VPNKillSwitchCard from './VPNKillSwitchCard.svelte';

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
  return render(VPNKillSwitchCard, { props: { tunnel: status } }).body.replace(/<!--.*?-->/g, '');
}

const enabledButton = (label: string) =>
  new RegExp(`<button(?![^>]*disabled="")[^>]*>${label}</button>`);
const disabledButton = (label: string) =>
  new RegExp(`<button[^>]*disabled=""[^>]*>${label}</button>`);

describe('VPN kill switch card', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockReturnValue(kill);
  });

  it('polls the kill switch state', () => {
    card({ enabled: false, zone: false }, tunnel('connected'));
    expect(createLoader).toHaveBeenCalledWith(fetchKillSwitch, 10000);
  });

  it('offers Enable only with a connected, fresh tunnel', () => {
    expect(card({ enabled: false, zone: false }, tunnel('connected'))).toMatch(
      enabledButton('Enable kill switch'),
    );
    for (const status of [
      tunnel('no_handshake'),
      tunnel('disabled'),
      tunnel('connected', true),
      undefined,
    ]) {
      expect(card({ enabled: false, zone: true }, status)).toMatch(
        disabledButton('Enable kill switch'),
      );
    }
  });

  it('always offers Disable when on and explains the effect', () => {
    const body = card({ enabled: true, zone: true }, tunnel('disabled'));
    expect(body).toMatch(enabledButton('Disable kill switch'));
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
