export const STATUS_KINDS = ['ok', 'warn', 'danger', 'info', 'neutral', 'stale'] as const;
export type StatusKind = (typeof STATUS_KINDS)[number];

/** CSS class pair for a status kind; colours live in tokens (app.css), never in pages. */
export function statusClass(kind: StatusKind): string {
  return `status status-${kind}`;
}
