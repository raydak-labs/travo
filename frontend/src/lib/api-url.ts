/**
 * Helpers for building API paths.
 *
 * Section names, interface names and service ids come from the router (and,
 * for ids, from list endpoints). Interpolating them raw lets a value such as
 * `../../system/reboot` or `a/b` escape the route it belongs to, so every
 * caller must encode the value.
 */

/** Percent-encodes a value used as a single path segment. */
export function encodePathSegment(value: string | number): string {
  return encodeURIComponent(String(value));
}

/** Fills a single `:param` placeholder in a route template with an encoded value. */
export function routeWithParam(route: string, value: string | number): string {
  return route.replace(/:[a-zA-Z]+/, encodePathSegment(value));
}

/** Appends an encoded path segment to a route. */
export function routeWithSegment(route: string, segment: string | number): string {
  return `${route}/${encodePathSegment(segment)}`;
}
