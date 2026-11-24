import { describe, expect, it } from 'vitest';
import {
  type AuthState,
  guard,
  href,
  NAV_GROUPS,
  type NavGroup,
  navGroup,
  parseHash,
  ROUTE_LIST,
  type Route,
  section,
} from './router';

const PATHS: [string, Route][] = [
  ['#/', 'dashboard'],
  ['#/login', 'login'],
  ['#/setup', 'setup'],
  ['#/internet', 'internet'],
  ['#/wifi', 'wifi'],
  ['#/vpn', 'vpn'],
  ['#/vpn/tailscale', 'vpn-tailscale'],
  ['#/clients', 'clients'],
  ['#/services', 'services'],
  ['#/network/wan', 'network-wan'],
  ['#/network/lan', 'network-lan'],
  ['#/network/dhcp', 'network-dhcp'],
  ['#/network/port-forwards', 'network-port-forwards'],
  ['#/network/ipv6', 'network-ipv6'],
  ['#/network/diagnostics', 'network-diagnostics'],
  ['#/system', 'system'],
  ['#/system/leds', 'system-leds'],
  ['#/system/logs', 'system-logs'],
  ['#/system/updates', 'system-updates'],
  ['#/system/tokens', 'system-tokens'],
  ['#/system/reset', 'system-reset'],
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

  it('opens WAN for the Network group path', () => {
    expect(parseHash('#/network')).toBe('network-wan');
  });

  it('falls back to the dashboard for unknown hashes', () => {
    expect(parseHash('#/nope')).toBe('dashboard');
    expect(parseHash('#/system/extra')).toBe('dashboard');
    expect(parseHash('#/network/nope')).toBe('dashboard');
  });

  it('highlights the parent for a child page that is not in the nav', () => {
    expect(section('vpn-tailscale')).toBe('vpn');
    expect(section('system-tokens')).toBe('system-tokens');
    expect(section('wifi')).toBe('wifi');
  });

  it.each(
    (Object.keys(NAV_GROUPS) as NavGroup[]).flatMap((group) =>
      NAV_GROUPS[group].map((route) => [route, group] as const),
    ),
  )('puts %s in the %s group', (route, group) => {
    expect(navGroup(section(route))).toBe(group);
  });

  it('keeps everyday pages out of the groups', () => {
    for (const route of [
      'dashboard',
      'internet',
      'wifi',
      'vpn',
      'vpn-tailscale',
      'clients',
      'services',
    ] as const) {
      expect(navGroup(route)).toBeNull();
    }
  });

  it('starts each group with its default page', () => {
    expect(NAV_GROUPS.network[0]).toBe('network-wan');
    expect(NAV_GROUPS.system[0]).toBe('system');
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
    expect(guard('anonymous', 'system-reset')).toBe('login');
    expect(guard('authenticated', 'setup')).toBe('dashboard');
    expect(guard('setup_required', 'network-lan')).toBe('setup');
  });
});
