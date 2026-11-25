export type Route =
  | 'dashboard'
  | 'login'
  | 'setup'
  | 'internet'
  | 'wifi'
  | 'vpn'
  | 'vpn-wireguard'
  | 'vpn-tailscale'
  | 'clients'
  | 'services'
  | 'services-adguard'
  | 'network-wan'
  | 'network-lan'
  | 'network-dhcp'
  | 'network-port-forwards'
  | 'network-ipv6'
  | 'network-diagnostics'
  | 'system'
  | 'system-hardware'
  | 'system-logs'
  | 'system-updates'
  | 'system-access'
  | 'system-reset';

/** Canonical path of every route; `href` builds these and `parseHash` reads them back. */
const PATHS: Record<Route, string> = {
  dashboard: '/',
  login: '/login',
  setup: '/setup',
  internet: '/internet',
  wifi: '/wifi',
  vpn: '/vpn',
  'vpn-wireguard': '/vpn/wireguard',
  'vpn-tailscale': '/vpn/tailscale',
  clients: '/clients',
  services: '/services',
  'services-adguard': '/services/adguard',
  'network-wan': '/network/wan',
  'network-lan': '/network/lan',
  'network-dhcp': '/network/dhcp',
  'network-port-forwards': '/network/port-forwards',
  'network-ipv6': '/network/ipv6',
  'network-diagnostics': '/network/diagnostics',
  system: '/system',
  'system-hardware': '/system/hardware',
  'system-logs': '/system/logs',
  'system-updates': '/system/updates',
  'system-access': '/system/access',
  'system-reset': '/system/reset',
};

// `#/network` has no page of its own: it opens the group's first page. Old paths keep working.
const ROUTES: Record<string, Route> = {
  ...Object.fromEntries(Object.entries(PATHS).map(([route, path]) => [path, route as Route])),
  '/network': 'network-wan',
  '/system/leds': 'system-hardware',
  '/system/tokens': 'system-access',
};

export const ROUTE_LIST = Object.keys(PATHS) as Route[];

export type NavGroup = 'vpn' | 'services' | 'network' | 'system';

/** Children of each collapsible nav group, in nav order. */
export const NAV_GROUPS: Record<NavGroup, readonly Route[]> = {
  vpn: ['vpn-wireguard', 'vpn-tailscale'],
  services: ['services-adguard'],
  network: [
    'network-wan',
    'network-lan',
    'network-dhcp',
    'network-port-forwards',
    'network-ipv6',
    'network-diagnostics',
  ],
  system: ['system-hardware', 'system-logs', 'system-updates', 'system-access', 'system-reset'],
};

/** The page a group label opens; a group without one (Network) only expands. */
export const GROUP_OVERVIEW: Record<NavGroup, Route | null> = {
  vpn: 'vpn',
  services: 'services',
  network: null,
  system: 'system',
};

/** The nav group a route belongs to (its overview or a child), or null for an everyday page. */
export function navGroup(route: Route): NavGroup | null {
  for (const group of Object.keys(NAV_GROUPS) as NavGroup[]) {
    if (GROUP_OVERVIEW[group] === route || NAV_GROUPS[group].includes(route)) return group;
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
