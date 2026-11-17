export type Route = 'dashboard' | 'login' | 'setup' | 'wifi' | 'vpn' | 'services' | 'system';

const ROUTES: Record<string, Route> = {
  '/': 'dashboard',
  '/login': 'login',
  '/setup': 'setup',
  '/wifi': 'wifi',
  '/vpn': 'vpn',
  '/services': 'services',
  '/system': 'system',
};

/** Maps a location hash (`#/wifi`) to a route; anything unknown is the dashboard. */
export function parseHash(hash: string): Route {
  const path = hash.replace(/^#/, '').split('?')[0] ?? '';
  return ROUTES[path === '' ? '/' : path] ?? 'dashboard';
}

export function href(route: Route): string {
  return route === 'dashboard' ? '#/' : `#/${route}`;
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
