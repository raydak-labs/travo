/**
 * Helpers for the unified access-point form: how the bands are ordered and
 * named, and whether the APs still share one set of credentials.
 */
import type { APConfig } from '@shared/index';
import { normalizeApEncryption } from './ap-config-normalize';
import { ApApplyRollbackError } from '@/lib/ap-section-apply';

export function bandSortKey(band: string): number {
  if (band === '2g') return 0;
  if (band === '5g') return 1;
  if (band === '6g') return 2;
  return 9;
}

export function pickPrimaryAp(aps: APConfig[]): APConfig {
  return [...aps].sort((a, b) => bandSortKey(a.band) - bandSortKey(b.band))[0]!;
}

export function credentialsMatchAcross(aps: APConfig[]): boolean {
  if (aps.length <= 1) return true;
  const p = pickPrimaryAp(aps);
  const enc0 = normalizeApEncryption(p.encryption);
  return aps.every(
    (a) => a.ssid === p.ssid && normalizeApEncryption(a.encryption) === enc0 && a.key === p.key,
  );
}

/**
 * Whether a failed apply left the router on the settings it had before.
 *
 * A restore that failed too means the router is holding a half-applied config,
 * which the operator has to see spelled out instead of a dialog that promises
 * nothing has changed yet.
 */
export function rollbackIsComplete(error: unknown): boolean {
  return !(error instanceof ApApplyRollbackError) || error.rollback.failed.length === 0;
}
