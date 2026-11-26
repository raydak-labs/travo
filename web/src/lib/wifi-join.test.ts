import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { Network, TxnBody } from './api/gen';
import WiFiJoinDialog from './pages/WiFiJoinDialog.svelte';
import WiFiScanList from './pages/WiFiScanList.svelte';
import { joinError, reduceJoin } from './wifi-join';

const network: Network = {
  ssid: 'Uplink',
  security: 'wpa2',
  access_points: [],
  bands: ['5g'],
  best_signal_dbm: -50,
};
const pending: TxnBody = {
  id: '1234567890abcdef',
  state: 'pending',
  started: '2026-11-13T22:47:16Z',
  configs: ['wireless'],
  summary: 'wifi: join Uplink',
  window_seconds: 60,
};

describe('WiFi join', () => {
  it('keeps the deadline while polling and uses the authoritative outcome', () => {
    const joining = reduceJoin({ state: 'idle' }, { type: 'start', ssid: network.ssid });
    expect(joining).toEqual({ state: 'joining', ssid: network.ssid });
    expect(reduceJoin({ state: 'idle' }, { type: 'start', ssid: 'AP', ap: true })).toEqual({
      state: 'joining',
      ssid: 'AP',
      ap: true,
    });
    const armed = reduceJoin(joining, { type: 'transaction', now: 1000, transaction: pending });
    expect(armed).toMatchObject({ state: 'pending', deadline: 61000 });
    expect(reduceJoin(armed, { type: 'transaction', now: 3000, transaction: pending })).toEqual(
      armed,
    );
    for (const state of ['restoring', 'confirmed', 'rolled_back', 'rollback_failed'] as const) {
      expect(
        reduceJoin(armed, { type: 'transaction', now: 3000, transaction: { ...pending, state } }),
      ).toMatchObject({ state });
    }
    expect(reduceJoin(armed, { type: 'failure', code: 'transaction_not_found' })).toEqual({
      state: 'failed',
      code: 'transaction_not_found',
    });
  });

  it('requires the key for WPA and hides the field on open networks', () => {
    const props = { network, close: () => {}, submit: () => {} };
    expect(render(WiFiJoinDialog, { props }).body).toContain('type="password"');
    expect(
      render(WiFiJoinDialog, { props: { ...props, network: { ...network, security: 'open' } } })
        .body,
    ).not.toContain('type="password"');
  });

  it('offers Save as a separate explicit action only when saving is wired', () => {
    const props = { network, close: () => {}, submit: () => {} };
    expect(render(WiFiJoinDialog, { props }).body).not.toContain('Save');
    const body = render(WiFiJoinDialog, {
      props: { ...props, save: async () => undefined },
    }).body;
    expect(body).toContain('Save');
    expect(body).toContain('type="submit"');
  });

  it('disables joining unsupported networks', () => {
    const { body } = render(WiFiScanList, {
      props: {
        join: () => {},
        result: { networks: [{ ...network, security: 'other' }], hidden: [], radios: [] },
      },
    });
    expect(body).toMatch(/<button[^>]* disabled[ =>]/);
    expect(body).toContain(joinError('wifi_security_unsupported'));
  });

  it('distinguishes named errors and falls back for unexpected codes', () => {
    const codes = [
      'wifi_no_free_radio',
      'transaction_pending',
      'wifi_security_unsupported',
      'wifi_key_invalid',
      'wifi_network_not_found',
      'wifi_lockout_risk',
      'wifi_no_change',
      'wifi_unavailable',
      'wifi_radio_in_use',
      'wifi_ap_not_found',
      'wifi_ssid_invalid',
    ];
    expect(new Set(codes.map(joinError)).size).toBe(codes.length);
    expect(joinError('wifi_key_required')).toBe(joinError('wifi_key_invalid'));
    expect(joinError('unknown')).toBe(joinError('wifi_unavailable'));
  });
});
