import { describe, expect, it } from 'vitest';
import { type AuthState, guard, href, parseHash, type Route, section } from './router';

describe('router', () => {
  it.each([
    ['', 'dashboard'],
    ['#/', 'dashboard'],
    ['#/login', 'login'],
    ['#/setup', 'setup'],
    ['#/wifi', 'wifi'],
    ['#/vpn', 'vpn'],
    ['#/system', 'system'],
    ['#/system?tab=x', 'system'],
    ['#/system/tokens', 'system-tokens'],
    ['#/vpn/tailscale', 'vpn-tailscale'],
  ])('maps %j to %s', (hash, route) => {
    expect(parseHash(hash)).toBe(route);
  });

  it('falls back to the dashboard for unknown hashes', () => {
    expect(parseHash('#/nope')).toBe('dashboard');
    expect(parseHash('#/system/extra')).toBe('dashboard');
  });

  it('builds hrefs that parse back to the same route', () => {
    for (const r of [
      'dashboard',
      'login',
      'setup',
      'wifi',
      'vpn',
      'system',
      'system-tokens',
      'vpn-tailscale',
    ] as const) {
      expect(parseHash(href(r))).toBe(r);
    }
  });

  it('builds the subpage href as a nested path', () => {
    expect(href('system-tokens')).toBe('#/system/tokens');
    expect(href('dashboard')).toBe('#/');
  });

  it('highlights the parent section for a subpage', () => {
    expect(section('system-tokens')).toBe('system');
    expect(section('vpn-tailscale')).toBe('vpn');
    expect(section('wifi')).toBe('wifi');
  });
});

describe('guard', () => {
  const routes: Route[] = [
    'dashboard',
    'login',
    'setup',
    'wifi',
    'network',
    'vpn',
    'services',
    'system',
    'system-tokens',
    'vpn-tailscale',
  ];
  const expected: Record<AuthState, Record<Route, Route>> = {
    unknown: {
      dashboard: 'dashboard',
      login: 'login',
      setup: 'setup',
      wifi: 'wifi',
      network: 'network',
      vpn: 'vpn',
      services: 'services',
      system: 'system',
      'system-tokens': 'system-tokens',
      'vpn-tailscale': 'vpn-tailscale',
    },
    setup_required: {
      dashboard: 'setup',
      login: 'setup',
      setup: 'setup',
      wifi: 'setup',
      network: 'setup',
      vpn: 'setup',
      services: 'setup',
      system: 'setup',
      'system-tokens': 'setup',
      'vpn-tailscale': 'setup',
    },
    anonymous: {
      dashboard: 'login',
      login: 'login',
      setup: 'login',
      wifi: 'login',
      network: 'login',
      vpn: 'login',
      services: 'login',
      system: 'login',
      'system-tokens': 'login',
      'vpn-tailscale': 'login',
    },
    authenticated: {
      dashboard: 'dashboard',
      login: 'dashboard',
      setup: 'dashboard',
      wifi: 'wifi',
      network: 'network',
      vpn: 'vpn',
      services: 'services',
      system: 'system',
      'system-tokens': 'system-tokens',
      'vpn-tailscale': 'vpn-tailscale',
    },
  };
  const cases = (Object.keys(expected) as AuthState[]).flatMap((state) =>
    routes.map((route) => [state, route, expected[state][route]] as const),
  );

  it.each(cases)('%s + %s -> %s', (state, route, want) => {
    expect(guard(state, route)).toBe(want);
  });
});
