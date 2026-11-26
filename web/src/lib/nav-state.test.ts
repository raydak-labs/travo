import { describe, expect, it } from 'vitest';
import {
  loadNavState,
  NAV_STATE_KEY,
  parseNavState,
  saveNavState,
  serializeNavState,
  withRoute,
} from './nav-state';

const collapsed = { vpn: false, services: false, network: false, system: false };

describe('nav state', () => {
  it('starts collapsed on a first visit', () => {
    expect(parseNavState(null)).toEqual(collapsed);
    expect(loadNavState({ getItem: () => null })).toEqual(collapsed);
  });

  it('opens the group of a child route and leaves everyday routes alone', () => {
    expect(withRoute(collapsed, 'network-port-forwards')).toEqual({ ...collapsed, network: true });
    expect(withRoute(collapsed, 'system-access')).toEqual({ ...collapsed, system: true });
    expect(withRoute(collapsed, 'vpn-tailscale')).toEqual({ ...collapsed, vpn: true });
    expect(withRoute(collapsed, 'services')).toEqual({ ...collapsed, services: true });
    expect(withRoute(collapsed, 'wifi')).toBe(collapsed);
  });

  it('returns the same state when the group is already open', () => {
    const open = { ...collapsed, network: true };
    expect(withRoute(open, 'network-lan')).toBe(open);
  });

  it('restores the stored state', () => {
    const stored = serializeNavState({ ...collapsed, vpn: true, system: true });
    expect(loadNavState({ getItem: (key) => (key === NAV_STATE_KEY ? stored : null) })).toEqual({
      ...collapsed,
      vpn: true,
      system: true,
    });
  });

  it.each([
    ['not json', '{'],
    ['other version', JSON.stringify({ version: 1, open: { network: true, system: true } })],
    ['no version', JSON.stringify({ open: { network: true } })],
    ['wrong shape', JSON.stringify({ version: 2, open: 'network' })],
    ['array', '[1]'],
    ['null', 'null'],
  ])('ignores a %s value', (_name, raw) => {
    expect(parseNavState(raw)).toEqual(collapsed);
  });

  it('treats non-boolean group values as closed', () => {
    expect(
      parseNavState(JSON.stringify({ version: 2, open: { network: 'yes', system: true } })),
    ).toEqual({ ...collapsed, system: true });
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
      { ...collapsed, network: true },
    );
    expect(writes).toEqual([
      [
        'nav:v2',
        '{"version":2,"open":{"vpn":false,"services":false,"network":true,"system":false}}',
      ],
    ]);
  });
});
