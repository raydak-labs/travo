/**
 * Validation for URLs that leave the app.
 *
 * A captive-portal URL or a Tailscale auth URL is attacker-influencable data
 * (anything answering on the upstream network can set it). Handing such a
 * value straight to `window.open` / `href` allows `javascript:` and
 * `data:` payloads to run in this origin, so every external navigation goes
 * through this allow-list first.
 */

const ALLOWED_PROTOCOLS = new Set(['http:', 'https:']);

/** True when the URL is an absolute http(s) URL we are willing to navigate to. */
export function isSafeExternalUrl(value: string | null | undefined): boolean {
  if (!value) return false;
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  return ALLOWED_PROTOCOLS.has(url.protocol);
}

/**
 * Opens an external URL in a new tab, isolated from this document.
 * Returns false (and opens nothing) for a URL outside the allow-list.
 */
export function openExternalUrl(value: string | null | undefined): boolean {
  if (!isSafeExternalUrl(value)) return false;
  window.open(value as string, '_blank', 'noopener,noreferrer');
  return true;
}
