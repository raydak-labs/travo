import type { GroupedScanNetwork, WifiBand } from '@shared/index';

export const DOWN_SWITCH_THRESHOLD_DBM = -70;

/** Normalize band to WifiBand for API (2.4ghz, 5ghz, 6ghz) */
export function toWifiBand(band: string): WifiBand {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4ghz';
  if (b === '5ghz' || b === '5g') return '5ghz';
  if (b === '6ghz' || b === '6g') return '6ghz';
  return band as WifiBand;
}

export function signalQuality(dbm: number): string {
  if (dbm >= -50) return 'Excellent';
  if (dbm >= -60) return 'Strong';
  if (dbm >= -70) return 'Good';
  if (dbm >= -80) return 'Fair';
  return 'Weak';
}

export type BandOption = { band: string; dbm: number };

export function buildBandOptionsFromGroup(group: GroupedScanNetwork): BandOption[] {
  const byBand = new Map<string, { dbm: number }>();
  for (const ap of group.aps) {
    const b = ap.band.toLowerCase();
    const key =
      b.includes('2.4') || b === '2.4g'
        ? '2.4ghz'
        : b.includes('5')
          ? '5ghz'
          : b.includes('6')
            ? '6ghz'
            : b;
    const existing = byBand.get(key);
    if (!existing || ap.signal_dbm > existing.dbm) {
      byBand.set(key, { dbm: ap.signal_dbm });
    }
  }
  return Array.from(byBand.entries()).map(([band, { dbm }]) => ({ band, dbm }));
}

export function pickDefaultBandFromOptions(bandOptions: BandOption[]): string | null {
  if (bandOptions.length <= 1) return bandOptions[0]?.band ?? null;
  const five = bandOptions.find((b) => b.band === '5ghz');
  const two = bandOptions.find((b) => b.band === '2.4ghz');
  if (five && five.dbm >= DOWN_SWITCH_THRESHOLD_DBM) return '5ghz';
  if (two) return '2.4ghz';
  return bandOptions[0]?.band ?? null;
}
