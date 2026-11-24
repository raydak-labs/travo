export type Route =
  | 'dashboard'
  | 'login'
  | 'setup'
  | 'internet'
  | 'wifi'
  | 'vpn'
  | 'vpn-tailscale'
  | 'clients'
  | 'services'
  | 'network-wan'
  | 'network-lan'
  | 'network-dhcp'
  | 'network-port-forwards'
  | 'network-ipv6'
  | 'network-diagnostics'
  | 'system'
  | 'system-leds'
  | 'system-logs'
  | 'system-updates'
  | 'system-tokens'
  | 'system-reset';

/** Canonical path of every route; `href` builds these and `parseHash` reads them back. */
const PATHS: Record<Route, string> = {
  dashboard: '/',
  login: '/login',
  setup: '/setup',
  internet: '/internet',
  wifi: '/wifi',
  vpn: '/vpn',
  'vpn-tailscale': '/vpn/tailscale',
  clients: '/clients',
  services: '/services',
  'network-wan': '/network/wan',
  'network-lan': '/network/lan',
  'network-dhcp': '/network/dhcp',
  'network-port-forwards': '/network/port-forwards',
  'network-ipv6': '/network/ipv6',
  'network-diagnostics': '/network/diagnostics',
  system: '/system',
  'system-leds': '/system/leds',
  'system-logs': '/system/logs',
  'system-updates': '/system/updates',
  'system-tokens': '/system/tokens',
  'system-reset': '/system/reset',
};

// `#/network` has no page of its own: it opens the group's default page.
const ROUTES: Record<string, Route> = {
  ...Object.fromEntries(Object.entries(PATHS).map(([route, path]) => [path, route as Route])),
  '/network': 'network-wan',
};

export const ROUTE_LIST = Object.keys(PATHS) as Route[];

export type NavGroup = 'network' | 'system';

/** Children of each collapsible nav group, in nav order; the first is the group's default page. */
export const NAV_GROUPS: Record<NavGroup, readonly Route[]> = {
  network: [
    'network-wan',
    'network-lan',
    'network-dhcp',
    'network-port-forwards',
    'network-ipv6',
    'network-diagnostics',
  ],
  system: [
    'system',
    'system-leds',
    'system-logs',
    'system-updates',
    'system-tokens',
    'system-reset',
  ],
};

/** The nav item a route highlights: a child page that is not in the nav highlights its parent. */
export function section(route: Route): Route {
  return route === 'vpn-tailscale' ? 'vpn' : route;
}

/** The nav group a route's nav item sits in, or null for an everyday page. */
export function navGroup(route: Route): NavGroup | null {
  const item = section(route);
  for (const group of Object.keys(NAV_GROUPS) as NavGroup[]) {
    if (NAV_GROUPS[group].includes(item)) return group;
  }
  return null;
}

/** Maps a location hash (`#/wifi`) to a route; anything unknown is the dashboard. */
export function parseHash(hash: string): Route {
  const path = hash.replace(/^#/, '').split('?')[0] ?? '';
  return ROUTES[path === '' ? '/' : path] ?? 'dashboard';
}

export function href(route: Route): string {
  return `#${PATHS[route]}`;
}

/** Auth state as the guard sees it; `session.svelte.ts` owns the transitions. */
export type AuthState = 'unknown' | 'setup_required' | 'anonymous' | 'authenticated';

/**
 * The route to actually show. `unknown` keeps the route: the app shows a loader until bootstrap
 * resolves, so nothing is redirected on a guess.
 */
export function guard(state: AuthState, route: Route): Route {
  switch (state) {
    case 'unknown':
      return route;
    case 'setup_required':
      return 'setup';
    case 'anonymous':
      return route === 'login' ? route : 'login';
    case 'authenticated':
      return route === 'login' || route === 'setup' ? 'dashboard' : route;
  }
}
