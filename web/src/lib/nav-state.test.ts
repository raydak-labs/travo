import { describe, expect, it } from 'vitest';
import {
  loadNavState,
  NAV_STATE_KEY,
  parseNavState,
  saveNavState,
  serializeNavState,
  withRoute,
} from './nav-state';

const collapsed = { network: false, system: false };

describe('nav state', () => {
  it('starts collapsed on a first visit', () => {
    expect(parseNavState(null)).toEqual(collapsed);
    expect(loadNavState({ getItem: () => null })).toEqual(collapsed);
  });

  it('opens the group of a child route and leaves everyday routes alone', () => {
    expect(withRoute(collapsed, 'network-dhcp')).toEqual({ network: true, system: false });
    expect(withRoute(collapsed, 'system-tokens')).toEqual({ network: false, system: true });
    expect(withRoute(collapsed, 'wifi')).toBe(collapsed);
    expect(withRoute(collapsed, 'vpn-tailscale')).toBe(collapsed);
  });

  it('returns the same state when the group is already open', () => {
    const open = { network: true, system: false };
    expect(withRoute(open, 'network-wan')).toBe(open);
  });

  it('restores the stored state', () => {
    const stored = serializeNavState({ network: false, system: true });
    expect(loadNavState({ getItem: (key) => (key === NAV_STATE_KEY ? stored : null) })).toEqual({
      network: false,
      system: true,
    });
  });

  it.each([
    ['not json', '{'],
    ['other version', JSON.stringify({ version: 2, open: { network: true, system: true } })],
    ['no version', JSON.stringify({ open: { network: true } })],
    ['wrong shape', JSON.stringify({ version: 1, open: 'network' })],
    ['array', '[1]'],
    ['null', 'null'],
  ])('ignores a %s value', (_name, raw) => {
    expect(parseNavState(raw)).toEqual(collapsed);
  });

  it('treats non-boolean group values as closed', () => {
    expect(
      parseNavState(JSON.stringify({ version: 1, open: { network: 'yes', system: true } })),
    ).toEqual({
      network: false,
      system: true,
    });
  });

  it('survives storage that throws', () => {
    const broken = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    };
    expect(loadNavState(broken)).toEqual(collapsed);
    expect(() => saveNavState(broken, collapsed)).not.toThrow();
    expect(loadNavState(undefined)).toEqual(collapsed);
  });

  it('writes under the versioned key', () => {
    const writes: [string, string][] = [];
    saveNavState(
      { setItem: (key, value) => writes.push([key, value]) },
      { network: true, system: false },
    );
    expect(writes).toEqual([['nav:v1', '{"version":1,"open":{"network":true,"system":false}}']]);
  });
});
