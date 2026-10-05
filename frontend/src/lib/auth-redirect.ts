/**
 * Preserving the user's destination across an auth redirect.
 *
 * A deep link to a protected page must survive both ways the app can bounce
 * through `/login`: the router guard on navigation, and a mid-session 401. Both
 * used to hard-code `/dashboard`, so a bookmarked page silently became the
 * dashboard.
 */
const DEFAULT_POST_LOGIN_PATH = '/dashboard';

/** The current location as a same-origin relative path, safe to put in a query param. */
export function currentRelativeLocation(): string {
  if (typeof window === 'undefined') return DEFAULT_POST_LOGIN_PATH;
  return `${window.location.pathname}${window.location.search}`;
}

/**
 * Resolves a `?redirect=` value to an in-app path.
 *
 * Only same-origin relative paths are accepted: `//evil.example` and
 * `https://evil.example` are protocol-relative or absolute URLs that would
 * turn the login page into an open redirect, so they fall back to the default.
 */
export function safeRedirectTarget(value: string | undefined): string {
  if (!value) return DEFAULT_POST_LOGIN_PATH;
  if (!value.startsWith('/') || value.startsWith('//')) return DEFAULT_POST_LOGIN_PATH;
  // `/login` itself would bounce the user straight back here after login.
  if (value === '/login' || value.startsWith('/login?')) return DEFAULT_POST_LOGIN_PATH;
  return value;
}
