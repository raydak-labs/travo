import { describe, expect, it } from 'vitest';
import { type AuthState, guard, href, parseHash, type Route } from './router';

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
  ])('maps %j to %s', (hash, route) => {
    expect(parseHash(hash)).toBe(route);
  });

  it('falls back to the dashboard for unknown hashes', () => {
    expect(parseHash('#/nope')).toBe('dashboard');
    expect(parseHash('#/system/extra')).toBe('dashboard');
  });

  it('builds hrefs that parse back to the same route', () => {
    for (const r of ['dashboard', 'login', 'setup', 'wifi', 'vpn', 'system'] as const) {
      expect(parseHash(href(r))).toBe(r);
    }
  });
});

describe('guard', () => {
  const routes: Route[] = ['dashboard', 'login', 'setup', 'wifi', 'vpn', 'services', 'system'];
  const expected: Record<AuthState, Record<Route, Route>> = {
    unknown: {
      dashboard: 'dashboard',
      login: 'login',
      setup: 'setup',
      wifi: 'wifi',
      vpn: 'vpn',
      services: 'services',
      system: 'system',
    },
    setup_required: {
      dashboard: 'setup',
      login: 'setup',
      setup: 'setup',
      wifi: 'setup',
      vpn: 'setup',
      services: 'setup',
      system: 'setup',
    },
    anonymous: {
      dashboard: 'login',
      login: 'login',
      setup: 'login',
      wifi: 'login',
      vpn: 'login',
      services: 'login',
      system: 'login',
    },
    authenticated: {
      dashboard: 'dashboard',
      login: 'dashboard',
      setup: 'dashboard',
      wifi: 'wifi',
      vpn: 'vpn',
      services: 'services',
      system: 'system',
    },
  };
  const cases = (Object.keys(expected) as AuthState[]).flatMap((state) =>
    routes.map((route) => [state, route, expected[state][route]] as const),
  );

  it.each(cases)('%s + %s -> %s', (state, route, want) => {
    expect(guard(state, route)).toBe(want);
  });
});
