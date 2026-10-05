// The alert type and the section-name extraction live apart from the component
// so that file only exports a component: a module exporting both defeats React
// Fast Refresh, which is what the existing oxlint rule is there to prevent.

import type { Alert } from '@shared/index';

export const OPERATOR_EDIT_OVERWRITE_ALERT = 'failover_operator_edits_overwritten';

// How recently an alert has to have been published before the card may call it
// happening now. GET /api/v1/system/alerts is the last 50 alerts with no TTL,
// so a three-day-old takeover is still in the feed on every load; "you just
// saved" would then be a confidently wrong thing to tell the operator.
export const OVERWRITE_FRESH_WINDOW_MS = 2 * 60 * 1000;

// Alert.timestamp is unix millis (backend/internal/models/system.go). Values
// below this are not a plausible wall clock: a missing field arrives as 0 and a
// seconds-valued timestamp lands in 1970. Neither is evidence of a recent save.
const MIN_PLAUSIBLE_EPOCH_MS = 1e12;

// Slack for the router's clock running ahead of the browser's: a timestamp a
// little in the future is still "just now", not a reason to go vague.
const CLOCK_SKEW_MS = 60 * 1000;

/** The alert's epoch millis, or null when it cannot be believed as a time. */
export function alertEpochMs(alert: Alert): number | null {
  const at = alert.timestamp;
  return Number.isFinite(at) && at >= MIN_PLAUSIBLE_EPOCH_MS ? at : null;
}

/** Whether the alert is recent enough to describe in the present tense. */
export function isRecentAlert(alert: Alert, now: number = Date.now()): boolean {
  const at = alertEpochMs(alert);
  if (at === null) return false;
  const age = now - at;
  return age >= -CLOCK_SKEW_MS && age <= OVERWRITE_FRESH_WINDOW_MS;
}

// The alert carries the section only inside its message, the way
// FailoverService.adoptSection words it, so the name is lifted from there.
// Returns null when the message does not name a section: better a warning
// without a name than a name invented from something else.
export function adoptedSection(message: string): string | null {
  return /mwan3 section (\S+?):/.exec(message)?.[1] ?? null;
}

// Where the operator's "I have read this" lives. The feed is server-retained
// (the last 50 alerts, no TTL) and refetched on every mount, so a dismissal held
// in component state is gone the moment the operator navigates away: they are
// shown the same banner again on the next visit and have to clear it again.
// Keeping the dismissed IDs in local storage is the mirror half of that
// durability — an alert nobody has seen is re-shown, an alert they dismissed is
// not.
const DISMISSED_STORAGE_KEY = 'otg-dismissed-overwrite-alerts';

// The feed itself keeps 50 alerts, so remembering every dismissal forever would
// only grow storage for alerts the backend no longer serves.
const MAX_REMEMBERED_DISMISSALS = 50;

/** The alert IDs the operator has dismissed, oldest first. Never throws. */
export function dismissedOverwriteAlerts(): readonly string[] {
  try {
    const raw = window.localStorage.getItem(DISMISSED_STORAGE_KEY);
    const parsed: unknown = raw === null ? [] : JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((id): id is string => typeof id === 'string');
  } catch {
    // No storage (private mode, quota) is not a reason to re-show everything.
    return [];
  }
}

/**
 * Records a dismissal and returns the new list, so a component can hold the
 * result in state and render from the same array the next mount will read.
 */
export function rememberDismissedOverwriteAlert(alertId: string): readonly string[] {
  const next = [...dismissedOverwriteAlerts().filter((id) => id !== alertId), alertId].slice(
    -MAX_REMEMBERED_DISMISSALS,
  );
  try {
    window.localStorage.setItem(DISMISSED_STORAGE_KEY, JSON.stringify(next));
  } catch {
    /* ignore: the in-session state still hides the warning */
  }
  return next;
}
