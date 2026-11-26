import { describe, expect, it } from 'vitest';
import {
  type AuthState,
  GROUP_OVERVIEW,
  guard,
  href,
  NAV_GROUPS,
  type NavGroup,
  navGroup,
  parseHash,
  ROUTE_LIST,
  type Route,
} from './router';

const PATHS: [string, Route][] = [
  ['#/', 'dashboard'],
  ['#/login', 'login'],
  ['#/setup', 'setup'],
  ['#/internet', 'internet'],
  ['#/wifi', 'wifi'],
  ['#/vpn', 'vpn'],
  ['#/vpn/wireguard', 'vpn-wireguard'],
  ['#/vpn/tailscale', 'vpn-tailscale'],
  ['#/clients', 'clients'],
  ['#/services', 'services'],
  ['#/services/adguard', 'services-adguard'],
  ['#/network/lan', 'network-lan'],
  ['#/network/port-forwards', 'network-port-forwards'],
  ['#/network/ipv6', 'network-ipv6'],
  ['#/network/diagnostics', 'network-diagnostics'],
  ['#/system', 'system'],
  ['#/system/hardware', 'system-hardware'],
  ['#/system/logs', 'system-logs'],
  ['#/system/firmware', 'system-firmware'],
  ['#/system/access', 'system-access'],
];

describe('router', () => {
  it.each(PATHS)('maps %j to %s and back', (hash, route) => {
    expect(parseHash(hash)).toBe(route);
    expect(href(route)).toBe(hash);
  });

  it('covers every route', () => {
    expect(PATHS.map(([, route]) => route).sort()).toEqual([...ROUTE_LIST].sort());
  });

  it('ignores the query and treats an empty hash as the dashboard', () => {
    expect(parseHash('')).toBe('dashboard');
    expect(parseHash('#/system?tab=x')).toBe('system');
  });

  it('opens LAN for the Network group path', () => {
    expect(parseHash('#/network')).toBe('network-lan');
  });

  it('falls back to the dashboard for unknown hashes', () => {
    expect(parseHash('#/nope')).toBe('dashboard');
    expect(parseHash('#/system/extra')).toBe('dashboard');
    expect(parseHash('#/network/nope')).toBe('dashboard');
  });

  it('keeps old paths working', () => {
    expect(parseHash('#/system/leds')).toBe('system-hardware');
    expect(parseHash('#/system/tokens')).toBe('system-access');
    expect(parseHash('#/system/updates')).toBe('system-firmware');
    expect(parseHash('#/system/reset')).toBe('system-firmware');
    expect(parseHash('#/network/wan')).toBe('internet');
    expect(parseHash('#/network/dhcp')).toBe('network-lan');
  });

  it.each(
    (Object.keys(NAV_GROUPS) as NavGroup[]).flatMap((group) =>
      NAV_GROUPS[group].map((route) => [route, group] as const),
    ),
  )('puts %s in the %s group', (route, group) => {
    expect(navGroup(route)).toBe(group);
  });

  it('puts each overview in its group and has none for Network', () => {
    expect(navGroup('vpn')).toBe('vpn');
    expect(navGroup('services')).toBe('services');
    expect(navGroup('system')).toBe('system');
    expect(GROUP_OVERVIEW.network).toBeNull();
  });

  it('keeps everyday pages out of the groups', () => {
    for (const route of ['dashboard', 'internet', 'wifi', 'clients'] as const) {
      expect(navGroup(route)).toBeNull();
    }
  });
});

describe('guard', () => {
  const want = (state: AuthState, route: Route): Route => {
    if (state === 'unknown') return route;
    if (state === 'setup_required') return 'setup';
    if (state === 'anonymous') return 'login';
    return route === 'login' || route === 'setup' ? 'dashboard' : route;
  };
  const states: AuthState[] = ['unknown', 'setup_required', 'anonymous', 'authenticated'];
  const cases = states.flatMap((state) =>
    ROUTE_LIST.map((route) => [state, route, want(state, route)] as const),
  );

  it.each(cases)('%s + %s -> %s', (state, route, expected) => {
    expect(guard(state, route)).toBe(expected);
  });

  it('pins the redirects that matter', () => {
    expect(guard('anonymous', 'login')).toBe('login');
    expect(guard('anonymous', 'system-firmware')).toBe('login');
    expect(guard('authenticated', 'setup')).toBe('dashboard');
    expect(guard('setup_required', 'network-lan')).toBe('setup');
  });
});
