import { href, type Route } from './router';

// Own module: no browser globals at import time, so server-rendered page tests can load it.
/** Moves to a route. `replace` keeps guard redirects out of the history so Back does not loop. */
export function navigate(route: Route, { replace = false } = {}): void {
  if (replace) location.replace(href(route));
  else location.hash = href(route);
}
