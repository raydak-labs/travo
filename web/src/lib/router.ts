export type Route =
  | 'dashboard'
  | 'login'
  | 'setup'
  | 'wifi'
  | 'network'
  | 'vpn'
  | 'services'
  | 'system'
  | 'system-tokens';

const ROUTES: Record<string, Route> = {
  '/': 'dashboard',
  '/login': 'login',
  '/setup': 'setup',
  '/wifi': 'wifi',
  '/network': 'network',
  '/vpn': 'vpn',
  '/services': 'services',
  '/system': 'system',
  '/system/tokens': 'system-tokens',
};

const PATHS = Object.fromEntries(
  Object.entries(ROUTES).map(([path, route]) => [route, path]),
) as Record<Route, string>;

/** The nav entry a route belongs to: a subpage highlights its parent. */
export function section(route: Route): Route {
  return route === 'system-tokens' ? 'system' : route;
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
