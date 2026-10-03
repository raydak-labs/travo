import type { APConfig, APConfigUpdate } from '@shared/index';

/**
 * The state of one AP UCI section before a multi-section write started.
 *
 * Writing shared credentials to every band is a sequence of independent PUTs,
 * and each one is committed and confirmed on its own. Without a snapshot of
 * what was on the device before the first PUT, a failure on the second section
 * leaves the router with two different SSIDs/keys and the user no way back
 * except retyping the old values by hand.
 */
export interface ApSectionSnapshot {
  readonly section: string;
  readonly radio: string;
  readonly band: string;
  readonly config: APConfigUpdate;
}

export interface ApRollbackResult {
  /** Sections whose previous configuration was written back successfully. */
  readonly restored: string[];
  /** Sections whose restore attempt failed too — these still hold the new values. */
  readonly failed: string[];
}

export function apBandLabel(band: string): string {
  if (band === '2g') return '2.4 GHz';
  if (band === '5g') return '5 GHz';
  if (band === '6g') return '6 GHz';
  return band;
}

/** Human-readable identification of a section for failure messages, e.g. "5 GHz (default_radio1)". */
export function describeApSection(ap: Pick<APConfig, 'radio' | 'band' | 'section'>): string {
  const label = apBandLabel(ap.band);
  return ap.radio ? `${label} ${ap.radio} (${ap.section})` : `${label} (${ap.section})`;
}

export function snapshotApSection(ap: APConfig): ApSectionSnapshot {
  return {
    section: ap.section,
    radio: ap.radio,
    band: ap.band,
    config: {
      ssid: ap.ssid,
      encryption: ap.encryption,
      key: ap.key,
      enabled: ap.enabled,
    },
  };
}

export function snapshotApSections(aps: readonly APConfig[]): ApSectionSnapshot[] {
  return aps.map(snapshotApSection);
}

/**
 * Re-applies the snapshot of every already-written section, newest first, so
 * the last change is undone before the earlier ones.
 *
 * A restore that fails is recorded rather than thrown: the caller already has
 * the original failure to report, and the sections that could not be restored
 * must still be named for the operator.
 */
export async function rollbackApSections(
  written: readonly ApSectionSnapshot[],
  apply: (snapshot: ApSectionSnapshot) => Promise<unknown>,
): Promise<ApRollbackResult> {
  const restored: string[] = [];
  const failed: string[] = [];
  for (const snapshot of [...written].reverse()) {
    try {
      await apply(snapshot);
      restored.push(snapshot.section);
    } catch {
      failed.push(snapshot.section);
    }
  }
  return { restored, failed };
}

/**
 * A multi-section apply that failed on `ap` after `written` sections had
 * already been committed, with the rollback outcome attached so the caller can
 * tell the operator exactly what the router is left holding.
 */
export class ApApplyRollbackError extends Error {
  readonly section: string;
  readonly cause: unknown;
  readonly rollback: ApRollbackResult;

  constructor(
    ap: Pick<APConfig, 'radio' | 'band' | 'section'>,
    cause: unknown,
    rollback: ApRollbackResult,
  ) {
    super(`Failed to save ${describeApSection(ap)}`);
    this.name = 'ApApplyRollbackError';
    this.section = ap.section;
    this.cause = cause;
    this.rollback = rollback;
  }
}

function causeMessage(cause: unknown): string {
  if (cause instanceof Error && cause.message) return `: ${cause.message}`;
  return '';
}

/**
 * A one-line report for the operator: which section failed, which sections were
 * put back, and which ones the operator still has to fix by hand.
 */
export function describeApApplyRollback(error: unknown): string {
  if (!(error instanceof ApApplyRollbackError)) {
    return error instanceof Error && error.message ? error.message : 'Unknown error';
  }
  const parts = [`${error.message}${causeMessage(error.cause)}.`];
  if (error.rollback.restored.length > 0) {
    parts.push(`Rolled back to the previous settings on ${error.rollback.restored.join(', ')}.`);
  }
  if (error.rollback.failed.length > 0) {
    parts.push(
      `Could not roll back ${error.rollback.failed.join(', ')} — it still has the new settings.`,
    );
  }
  return parts.join(' ');
}
