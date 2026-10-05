import { AlertTriangle, X } from 'lucide-react';
import type { Alert } from '@shared/index';
import { adoptedSection, alertEpochMs, isRecentAlert } from './operator-edit-overwrite';

interface Props {
  alert: Alert;
  onDismiss: (alertId: string) => void;
}

/**
 * Shown on the card the save was made from, not only in the header bell: the
 * operator's own tuning was replaced under their hands, and that is only
 * actionable while they are looking at the failover settings.
 */
export function OperatorEditOverwriteWarning({ alert, onDismiss }: Props) {
  const section = adoptedSection(alert.message);
  // Only an alert recent enough to justify it is described as the operator's
  // own save; an older one is stated as a takeover at a time, which is the only
  // thing still true once the save has scrolled out of this session.
  const when = alertEpochMs(alert);
  const replacedBy = isRecentAlert(alert)
    ? 'the settings you just saved.'
    : `the settings saved ${when ? new Date(when).toLocaleString() : 'at an unrecorded time'}.`;

  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-[var(--status-warn-border)] bg-[var(--status-warn-surface)] p-3 text-sm text-[var(--status-warn-text)]"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 flex-1 space-y-1">
        <div className="font-medium">
          {section ? (
            <>
              Travo adopted your mwan3 section <code>{section}</code>
            </>
          ) : (
            'Travo adopted an mwan3 section you had edited'
          )}
        </div>
        <p>
          The options Travo owns in that section were replaced by {replacedBy} Everything else you
          put there was kept.
        </p>
        {section ? (
          <p>
            Check what is live with{' '}
            <code className="rounded bg-[var(--status-warn-border)] px-1">
              uci show mwan3.{section}
            </code>{' '}
            and re-apply your tuning there. The next failover save replaces the same options again.
          </p>
        ) : null}
      </div>
      <button
        type="button"
        onClick={() => onDismiss(alert.id)}
        aria-label="Dismiss mwan3 section overwrite warning"
        className="shrink-0 rounded p-1 text-[var(--status-warn-text)] hover:bg-[var(--status-warn-border)]"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
    </div>
  );
}
