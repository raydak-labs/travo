/**
 * Status dot for "this link is up" / "this link is down".
 *
 * The colours come from the semantic status tokens rather than a palette
 * literal, so an up/down dot in the header, a client row or the DDNS panel is
 * the same green/red the StatusPill next to it uses, in both themes. The glow
 * is the same token: a second colour literal here would drift from the dot.
 */
export function statusDotClass(up: boolean): string {
  return up
    ? 'bg-[var(--status-ok-border)] shadow-[0_0_6px_var(--status-ok-border)]'
    : 'bg-[var(--status-danger-border)] shadow-[0_0_6px_var(--status-danger-border)]';
}

/** Neutral dot for a link that is simply not present or not applicable. */
export const statusDotIdleClass = 'bg-[var(--status-neutral-border)]';
